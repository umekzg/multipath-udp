package muxer

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/muxfd/multipath-udp/pkg/buffer"
	"github.com/muxfd/multipath-udp/pkg/srt"
)

type Muxer struct {
	buf *buffer.CompositeReceiverBuffer

	responseCh chan []byte
}

// NewMuxer creates a new uniplex listener muxer
func NewMuxer(options ...func(*Muxer)) *Muxer {
	buf := buffer.NewCompositeReceiverBuffer(500*time.Millisecond, 500*time.Millisecond)
	m := &Muxer{buf: buf, responseCh: make(chan []byte, 128)}

	for _, option := range options {
		option(m)
	}

	return m
}

func (m *Muxer) Start(listen, dial *net.UDPAddr) {
	go m.readLoop(listen)
	go m.writeLoop(dial)
}

func (m *Muxer) writeLoop(dial *net.UDPAddr) {
	w, err := net.DialUDP("udp", nil, dial)
	if err != nil {
		panic(err)
	}
	w.SetReadBuffer(1024 * 1024)
	w.SetWriteBuffer(1024 * 1024)

	go func() {
		for {
			msg := make([]byte, 2048)
			n, err := w.Read(msg)
			if err != nil {
				fmt.Printf("failed to read response %v\n", err)
				break
			}

			m.responseCh <- msg[:n]
		}
	}()

	for {
		p, ok := <-m.buf.EmitCh
		if !ok {
			fmt.Printf("emit ch closed\n")
			break
		}

		if _, err := w.Write(p.Marshal()); err != nil {
			fmt.Printf("failed to write packet %v\n", err)
		}
	}
}

func (m *Muxer) readLoop(listen *net.UDPAddr) {
	r, err := net.ListenUDP("udp", listen)
	if err != nil {
		panic(err)
	}
	r.SetReadBuffer(1024)
	r.SetWriteBuffer(1024)

	senders := make(map[string]*net.UDPAddr)
	handshaken := false
	var senderLock sync.Mutex

	go func() {
		for {
			select {
			case msg, ok := <-m.responseCh:
				if !ok {
					fmt.Printf("response ch closed\n")
					break
				}

				// broadcast to all senders since this is a control packet.
				senderLock.Lock()
				p, err := srt.Unmarshal(msg)
				if err != nil {
					fmt.Printf("not an srt packet: %v\n", err)
					continue
				}
				switch v := p.(type) {
				case *srt.DataPacket:
					fmt.Printf("recv pkt %d\n", v.SequenceNumber())
					for _, sender := range senders {
						if _, err := r.WriteToUDP(msg, sender); err != nil {
							fmt.Printf("sender %v closed\n", sender)
							delete(senders, sender.String())
						}
					}
				case *srt.ControlPacket:
					// pick a random socket.
					if v.ControlType() == srt.ControlTypeHandshake {
						handshaken = true
					}
					i := rand.Intn(len(senders))
					for _, sender := range senders {
						if i == 0 {
							if _, err := r.WriteToUDP(msg, sender); err != nil {
								fmt.Printf("sender %v closed\n", sender)
								delete(senders, sender.String())
							}
							break
						} else {
							i--
						}
					}
				}
				senderLock.Unlock()
			case msg := <-m.buf.MissingCh:
				senderLock.Lock()
				i := rand.Intn(len(senders))
				for _, sender := range senders {
					if i == 0 {
						if _, err := r.WriteToUDP(msg.RawPacket, sender); err != nil {
							fmt.Printf("sender %v closed\n", sender)
							delete(senders, sender.String())
						}
						break
					} else {
						i--
					}
				}
				senderLock.Unlock()

			}
		}
	}()

	// measure bitrate in 2-second blocks.
	expiration := time.Now().Add(1 * time.Second)
	counts := make(map[string]int)

	for {
		msg := make([]byte, 2048)
		start := time.Now()
		n, senderAddr, err := r.ReadFromUDP(msg)
		if err != nil {
			fmt.Printf("error reading %v\n", err)
			break
		}
		end := time.Now()

		fmt.Printf("read delay %d\n", end.Sub(start).Microseconds())

		if _, ok := senders[senderAddr.String()]; !ok {
			senderLock.Lock()
			senders[senderAddr.String()] = senderAddr
			senderLock.Unlock()
		}

		if expiration.Before(time.Now()) {
			// broadcast statistics downstream.
			if handshaken {
				go func(counts map[string]int) {
					for senderAddr, ct := range counts {
						fmt.Printf("%v recv ct %v\n", senderAddr, ct)
						p := srt.NewMultipathAckControlPacket(uint32(ct))
						senderLock.Lock()
						if sender, ok := senders[senderAddr]; ok {
							r.WriteToUDP(p.Marshal(), sender)
						}
						senderLock.Unlock()
					}
				}(counts)
			}
			counts = make(map[string]int)
			expiration = time.Now().Add(1 * time.Second)
		}

		counts[senderAddr.String()] += 1

		p, err := srt.Unmarshal(msg[:n])
		if err != nil {
			fmt.Printf("error unmarshalling rtp packet %v\n", err)
			continue
		}

		m.buf.Add(p)
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (m *Muxer) Close() {
	m.buf.Close()
	close(m.responseCh)
}
