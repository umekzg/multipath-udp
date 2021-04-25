package demuxer

import (
	"fmt"
	"net"

	"github.com/muxfd/multipath-udp/pkg/interfaces"
)

// Demuxer represents a UDP stream demuxer that demuxes a source over multiple senders.
type Demuxer struct {
	responseCh chan *Message

	interfaceBinder *interfaces.AutoBinder
}

// NewDemuxer creates a new demuxer.
func NewDemuxer(options ...func(*Demuxer)) *Demuxer {
	d := &Demuxer{responseCh: make(chan *Message, 1024)}

	for _, option := range options {
		option(d)
	}

	return d
}

func (d *Demuxer) Start(listen, dial *net.UDPAddr) {
	go d.readLoop(listen, dial)
}

func (d *Demuxer) readLoop(listen, dial *net.UDPAddr) {
	r, err := net.ListenUDP("udp", listen)
	if err != nil {
		panic(err)
	}
	r.SetReadBuffer(1024)
	r.SetWriteBuffer(1024)

	interfaces := NewInterfaceSet(dial, d.responseCh)
	if d.interfaceBinder != nil {
		close := d.interfaceBinder.Bind(interfaces.Add, interfaces.Remove, dial)
		defer close()
	}

	sessions := make(map[string]chan []byte)

	for {
		msg := make([]byte, 1500)

		n, senderAddr, err := r.ReadFromUDP(msg)
		if err != nil {
			fmt.Printf("error reading %v\n", err)
			break
		}

		if session, ok := sessions[senderAddr.String()]; ok {
			session <- msg[:n]
		} else {
			reqCh := make(chan []byte, 128)
			sessions[senderAddr.String()] = reqCh
			session <- msg[:n]

			go func(reqCh chan []byte, senderAddr *net.UDPAddr) {
				conns := interfaces.Connections()
				if len(conns) == 0 {
					fmt.Printf("no connections available\n")
					return
				}

				go func() {
					for {
						msg, ok := <-reqCh
						if !ok {
							break
						}
						for _, conn := range conns {
							if _, err := conn.Write(msg); err != nil {
								fmt.Printf("error writing to socket %v: %v\n", conn, err)
							}
						}
					}
				}()

				go func() {
					for {
						msg, ok := <-d.responseCh
						if !ok {
							break
						}
						if n, err := r.WriteToUDP(msg.msg, senderAddr); err != nil || n != len(msg.msg) {
							fmt.Printf("error writing to udp socket %v\n", err)
						}
					}
				}()
			}(reqCh, senderAddr)
		}

		// p, err := srt.Unmarshal(msg[:n])
		// if err != nil {
		// 	fmt.Printf("error unmarshaling srt packet %v\n", err)
		// 	return
		// }
		// if p.DestinationSocketId() == 0 {
		// 	switch v := p.(type) {
		// 	case *srt.ControlPacket:
		// 		sourceLock.Lock()
		// 		sources[v.HandshakeSocketId()] = senderAddr
		// 		sourceLock.Unlock()
		// 	}
		// }
		// if err != nil {
		// 	fmt.Printf("error unmarshalling rtp packet %v\n", err)
		// 	return
		// }

		// for _, conn := range conns {
		// 	if _, err := conn.Write(msg[:n]); err != nil {
		// 		fmt.Printf("error writing to socket %v: %v\n", conn, err)
		// 	}
		// }

		// switch v := p.(type) {
		// case *srt.DataPacket:
		// 	fmt.Printf("sending pkt %d\n", v.SequenceNumber())
		// 	// write to all interfaces.
		// 	for _, conn := range conns {
		// 		if _, err := conn.Write(msg[:n]); err != nil {
		// 			fmt.Printf("error writing to socket %v: %v\n", conn, err)
		// 		}
		// 	}
		// case *srt.ControlPacket:
		// 	// pick a random connection.
		// 	fmt.Printf("control pkt\n")
		// 	conn := conns[0]
		// 	if _, err := conn.Write(msg[:n]); err != nil {
		// 		fmt.Printf("error writing to socket %v: %v\n", conn, err)
		// 	}
		// }
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {
	close(d.responseCh)
}
