package muxer

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/muxfd/multipath-udp/pkg/buffer"
	"github.com/muxfd/multipath-udp/pkg/srt"
)

type Muxer struct {
}

// NewMuxer creates a new uniplex listener muxer
func NewMuxer(options ...func(*Muxer)) *Muxer {
	m := &Muxer{}

	for _, option := range options {
		option(m)
	}

	return m
}

func (m *Muxer) Start(listen, dial *net.UDPAddr) {
	go m.readLoop(listen, dial)
}

type Session struct {
	SRTConn *net.UDPConn
	buffer  *buffer.ReceiverBuffer
	sources []*net.UDPAddr
}

func NewSession(dial *net.UDPAddr) (*Session, error) {
	conn, err := net.DialUDP("udp", nil, dial)
	if err != nil {
		return nil, err
	}

	return &Session{
		SRTConn: conn,
		buffer:  buffer.NewReceiverBuffer(500 * time.Millisecond),
		sources: make([]*net.UDPAddr, 0, 5),
	}, nil
}

func (s *Session) AddSender(senderAddr *net.UDPAddr) {
	for _, addr := range s.sources {
		if addr.Port == senderAddr.Port {
			return
		}
	}
	s.sources = append(s.sources, senderAddr)
}

func (m *Muxer) readLoop(listen, dial *net.UDPAddr) {
	r, err := net.ListenUDP("udp", listen)
	if err != nil {
		panic(err)
	}

	var sessionsLock sync.RWMutex
	sessions := make(map[uint32]*Session)
	// measure bitrate in 2-second blocks.
	// expiration := time.Now().Add(1 * time.Second)
	// counts := make(map[string]int)

	for {
		msg := make([]byte, 2048)
		n, senderAddr, err := r.ReadFromUDP(msg)
		if err != nil {
			fmt.Printf("error reading %v\n", err)
			break
		}

		// if expiration.Before(time.Now()) {
		// 	// broadcast statistics downstream.
		// 	go func(counts map[string]int) {
		// 		for senderAddr, ct := range counts {
		// 			fmt.Printf("%v recv ct %v\n", senderAddr, ct)
		// 			p := srt.NewMultipathAckControlPacket(uint32(ct))
		// 			senderLock.Lock()
		// 			if sender, ok := senders[senderAddr]; ok {
		// 				r.WriteToUDP(p.Marshal(), sender)
		// 			}
		// 			senderLock.Unlock()
		// 		}
		// 	}(counts)
		// 	counts = make(map[string]int)
		// 	expiration = time.Now().Add(1 * time.Second)
		// }

		// counts[senderAddr.String()] += 1

		p, err := srt.Unmarshal(msg[:n])
		if err != nil {
			fmt.Printf("error unmarshalling rtp packet %v\n", err)
			continue
		}

		switch v := p.(type) {
		case *srt.DataPacket:
			socketId := v.DestinationSocketId()
			sessionsLock.RLock()
			if session, ok := sessions[socketId]; ok {
				session.AddSender(senderAddr)
				session.buffer.Add(v)
			} else {
				fmt.Printf("data packet received before handshake for socket id %d\n", socketId)
			}
			sessionsLock.RUnlock()
		case *srt.ControlPacket:
			if v.ControlType() == srt.ControlTypeHandshake {
				socketId := v.HandshakeSocketId()
				sessionsLock.RLock()
				session, ok := sessions[socketId]
				sessionsLock.RUnlock()
				if !ok {
					session, err = NewSession(dial)
					if err != nil {
						fmt.Printf("failed to create session %v\n", err)
						break
					}

					go func() {
						var buffer [1500]byte
						for {
							n, err := session.SRTConn.Read(buffer[0:])
							if err != nil {
								fmt.Printf("srt conn closed %v\n", err)
								break
							}
							q, err := srt.Unmarshal(buffer[:n])
							if err != nil {
								break
							}
							switch z := q.(type) {
							case *srt.ControlPacket:
								if z.ControlType() == srt.ControlTypeHandshake {
									sessionsLock.Lock()
									sessions[z.HandshakeSocketId()] = session
									sessionsLock.Unlock()
								}
							}
							for _, senderAddr := range session.sources {
								if _, err := r.WriteToUDP(buffer[:n], senderAddr); err != nil {
									fmt.Printf("error writing response %v\n", err)
									break
								}
							}
						}
					}()

					go func() {
						for {
							msg, ok := <-session.buffer.EmitCh
							if !ok {
								break
							}
							if _, err := session.SRTConn.Write(msg.Marshal()); err != nil {
								fmt.Printf("error writing to srt %v\n", err)
							}
						}
					}()

					go func() {
						for {
							msg, ok := <-session.buffer.MissingCh
							if !ok {
								break
							}
							for _, senderAddr := range session.sources {
								if _, err := r.WriteToUDP(msg.Marshal(), senderAddr); err != nil {
									fmt.Printf("error writing response %v\n", err)
									break
								}
							}
						}
					}()
				}
				session.AddSender(senderAddr)
				if _, err := session.SRTConn.Write(v.Marshal()); err != nil {
					fmt.Printf("error writing to srt %v\n", err)
				}
			} else {
				socketId := v.DestinationSocketId()
				sessionsLock.RLock()
				if session, ok := sessions[socketId]; ok {
					session.AddSender(senderAddr)
					if _, err := session.SRTConn.Write(v.Marshal()); err != nil {
						fmt.Printf("error writing to srt %v\n", err)
					}
				} else {
					fmt.Printf("control packet received before handshake for socket id %d\n", socketId)
				}
				sessionsLock.RUnlock()
			}
		}

	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (m *Muxer) Close() {
}
