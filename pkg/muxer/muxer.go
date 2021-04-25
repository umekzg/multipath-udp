package muxer

import (
	"fmt"
	"net"
	"sync"
	"time"

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

func (m *Muxer) readLoop(listen, dial *net.UDPAddr) {
	r, err := net.ListenUDP("udp", listen)
	if err != nil {
		panic(err)
	}

	var sessionsLock sync.RWMutex
	sessions := make(map[uint32]*Session)
	// measure bitrate in 2-second blocks.
	meter := NewMeter(2 * time.Second)

	for {
		msg := make([]byte, 2048)
		n, senderAddr, err := r.ReadFromUDP(msg)
		if err != nil {
			fmt.Printf("error reading %v\n", err)
			break
		}

		if meter.IsExpired() {
			meter.Expire(func(sender *net.UDPAddr, ct uint32) {
				p := srt.NewMultipathAckControlPacket(ct)
				fmt.Printf("sender %v ct %d\n", sender, ct)
				r.WriteToUDP(p.Marshal(), sender)
			})
		}
		meter.Increment(senderAddr)

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
					// go func(dst uint32) {
					// 	for {
					// 		time.Sleep(750 * time.Millisecond)
					// 		if _, err = session.SRTConn.Write(srt.NewKeepAlivePacket(dst).Marshal()); err != nil {
					// 			fmt.Printf("error writing keep alive %v\n", err)
					// 			break
					// 		}
					// 	}
					// }(socketId)

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
