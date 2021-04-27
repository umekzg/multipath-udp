package demuxer

import (
	"encoding/binary"
	"fmt"
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
	r.SetReadBuffer(64 * 1024 * 1024)
	seq := uint32(0)

	sessions := make(map[string]*Session)

	for {
		var buf [1500]byte
		n, senderAddr, err := r.ReadFromUDP(buf[0:])
		if err != nil {
			fmt.Printf("read failed %v\n", err)
			break
		}
		p, err := srt.Unmarshal(buf[:n])
		if err != nil {
			fmt.Printf("not a valid srt packet\n")
			continue
		}

		saddr := senderAddr.String()
		session, found := sessions[saddr]
		if !found {
			respCh := make(chan *Message, 128)
			session = NewSession(dial, respCh)
			close := d.interfaceBinder.Bind(session.Add, session.Remove, dial)
			defer close()
			defer session.Close()

			sessions[saddr] = session

			go func() {
				nakTimestamps := make(map[uint32]bool)
			PACKET:
				for {
					resp, ok := <-respCh
					if !ok {
						fmt.Printf("response ch closed\n")
						break
					}
					p, err := srt.Unmarshal(resp.msg)
					if err != nil {
						fmt.Printf("invalid srt packet %v\n", err)
						continue
					}
					switch v := p.(type) {
					case *srt.ControlPacket:
						switch v.ControlType() {
						case srt.ControlTypeUserDefined:
							switch v.Subtype() {
							case srt.SubtypeMultipathNak:
								timestamp := v.Timestamp()
								if nakTimestamps[timestamp] {
									// already processed
									continue PACKET
								}
								nakTimestamps[timestamp] = true
								severity := v.TypeSpecificInformation()
								from := binary.BigEndian.Uint32(v.RawPacket[16:20])
								to := binary.BigEndian.Uint32(v.RawPacket[20:24])
								fmt.Printf("nak %d-%d severity %d\n", from, to, severity)
								for i := from; i < to; i++ {
									pkt := session.buffer.Get(i)
									if pkt == nil {
										fmt.Printf("failed to fulfill nak %d, packet didn't exist\n", i)
										if _, err := r.WriteToUDP(
											srt.NewNakSingleControlPacket(session.socketId, i).Marshal(),
											senderAddr,
										); err != nil {
											fmt.Printf("error writing short nak\n")
										}
										continue
									} else if pkt.Data.SequenceNumber() != i {
										fmt.Printf("failed to fulfill nak %d, packet elided with %d (diff %d)\n", i, pkt.Data.SequenceNumber(), pkt.Data.SequenceNumber()-i)
										continue
									}
									// find out who this packet belonged to, dock a point from them.
									session.Deduct(pkt.Sender)
									for _, conn := range session.Connections() {
										if _, err = conn.Write(pkt.Data.Marshal()); err != nil {
											fmt.Printf("error writing pkt %v\n", err)
										}
									}
								}
							}
						case srt.ControlTypeHandshake:
							fmt.Printf("handshake %d -> %d\n", v.DestinationSocketId(), v.HandshakeSocketId())
							fallthrough
						default:
							if n, err := r.WriteToUDP(p.Marshal(), senderAddr); err != nil || n != len(p.Marshal()) {
								fmt.Printf("error writing response %v\n", err)
								continue PACKET
							}
						}
					case *srt.DataPacket:
						if _, err = r.WriteToUDP(p.Marshal(), senderAddr); err != nil {
							fmt.Printf("error writing response %v\n", err)
							continue PACKET
						}
					}
				}
			}()
		}

		switch v := p.(type) {
		case *srt.DataPacket:
			conn := session.ChooseConnection()
			addr := conn.LocalAddr().(*net.UDPAddr)
			if v.SequenceNumber() > seq+1 {
				// emit nak immediately, localhost -> localhost is usually reliably ordered
				// and if it's not it's cheap to send so whatever.
				fmt.Printf("short circuit nak %d-%d (%d)\n", seq+1, v.SequenceNumber()-1, v.SequenceNumber()-seq-1)
				if _, err := r.WriteToUDP(
					srt.NewNakRangeControlPacket(session.socketId, seq+1, v.SequenceNumber()-1).Marshal(),
					senderAddr,
				); err != nil {
					fmt.Printf("error writing short nak\n")
				}
			} else if v.SequenceNumber() > seq {
				// might be an out of order retransmission.
				seq = v.SequenceNumber()
			}
			session.buffer.Add(addr, v)
			if _, err = conn.Write(buf[:n]); err != nil {
				fmt.Printf("error writing pkt %v\n", err)
				session.Deduct(conn.LocalAddr().(*net.UDPAddr))
			}
		case *srt.ControlPacket:
			for _, conn := range session.Connections() {
				if _, err = conn.Write(buf[:n]); err != nil {
					fmt.Printf("error writing pkt %v\n", err)
					session.Deduct(conn.LocalAddr().(*net.UDPAddr))
				}
			}
		}
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {
}
