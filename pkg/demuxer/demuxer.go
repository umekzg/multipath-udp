package demuxer

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"

	"github.com/muxfd/multipath-udp/pkg/interfaces"
	"github.com/muxfd/multipath-udp/pkg/srt"
)

// Demuxer represents a UDP stream demuxer that demuxes a source over multiple senders.
type Demuxer struct {
	interfaceBinder *interfaces.AutoBinder
}

// NewDemuxer creates a new demuxer.
func NewDemuxer(options ...func(*Demuxer)) *Demuxer {
	d := &Demuxer{}

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

	sessions := make(map[string]*InterfaceSet)

	respCh := make(chan *Message, 128)

	for {
		var buffer [1500]byte
		n, senderAddr, err := r.ReadFromUDP(buffer[0:])
		if err != nil {
			break
		}
		p, err := srt.Unmarshal(buffer[:n])
		if err != nil {
			fmt.Printf("not a valid srt packet\n")
			continue
		}
		saddr := senderAddr.String()
		session, found := sessions[saddr]
		if !found {
			fmt.Printf("new session\n")
			session = NewInterfaceSet(dial, respCh)
			close := d.interfaceBinder.Bind(session.Add, session.Remove, dial)
			defer close()
			defer session.Close()

			sessions[saddr] = session

			go func() {
				for {
					resp, ok := <-respCh
					if !ok {
						break
					}
					p, err := srt.Unmarshal(resp.msg)
					if err != nil {
						fmt.Printf("invalid srt packet %v\n", err)
						continue
					}
					switch v := p.(type) {
					case *srt.ControlPacket:
						if v.ControlType() == srt.ControlTypeUserDefined && v.Subtype() == srt.SubtypeMultipathAck {
							// data payload
							fmt.Printf("recv metrics\n")
						} else if v.ControlType() == srt.ControlTypeUserDefined && v.Subtype() == srt.SubtypeMultipathNak {
							// data payload
							from := binary.BigEndian.Uint32(v.RawPacket[16:20])
							to := binary.BigEndian.Uint32(v.RawPacket[20:24])
							for i := from; i < to; i++ {
								msg := session.buffer.Get(i)
								if msg == nil || msg.SequenceNumber() != i {
									fmt.Printf("failed to fulfill nak %d\n", i)
									break
								}
								for _, conn := range session.Connections() {
									if _, err = conn.Write(msg.Marshal()); err != nil {
										fmt.Printf("error writing pkt %v\n", err)
									}
								}
							}
						} else {
							if v.ControlType() == srt.ControlTypeHandshake {
								fmt.Printf("handshake resp v%d %d %d %d\n", binary.BigEndian.Uint32(v.RawPacket[16:20]), v.DestinationSocketId(), v.HandshakeSocketId(), len(v.RawPacket))
							}
							if _, err = r.WriteToUDP(p.Marshal(), senderAddr); err != nil {
								fmt.Printf("error writing response %v\n", err)
								break
							}
						}
					case *srt.DataPacket:
						if _, err = r.WriteToUDP(p.Marshal(), senderAddr); err != nil {
							fmt.Printf("error writing response %v\n", err)
							break
						}
					}
				}
			}()
		}

		switch v := p.(type) {
		case *srt.DataPacket:
			session.buffer.Add(v)

			for _, conn := range session.Connections() {
				if _, err = conn.Write(buffer[:n]); err != nil {
					fmt.Printf("error writing pkt %v\n", err)
				}
			}
		case *srt.ControlPacket:
			if v.ControlType() == srt.ControlTypeHandshake {
				fmt.Printf("handshake v%d %d %d %d\n", binary.BigEndian.Uint32(v.RawPacket[16:20]), v.DestinationSocketId(), v.HandshakeSocketId(), len(v.RawPacket))
			}
			conns := session.Connections()
			conn := conns[rand.Intn(len(conns))]
			if _, err = conn.Write(buffer[:n]); err != nil {
				fmt.Printf("error writing pkt %v\n", err)
			}
		}
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {
}
