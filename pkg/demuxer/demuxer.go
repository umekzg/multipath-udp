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
							case srt.SubtypeMultipathAck:
								session.SetRecvCount(resp.addr, v.TypeSpecificInformation())
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
								for i := from; i < to; i++ {
									msg := session.buffer.Get(i)
									if msg == nil || msg.SequenceNumber() != i {
										fmt.Printf("failed to fulfill nak %d\n", i)
										continue
									}
									count := session.NumConnections() - int(severity)
									if count < 1 {
										count = 1
									}
									for _, conn := range session.Connections(count) {
										if _, err = conn.Write(msg.Marshal()); err != nil {
											fmt.Printf("error writing pkt %v\n", err)
										}
									}
								}
							}
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

		// check the meters.
		if session.sendMeter.IsExpired() {
			session.sendMeter.Expire(session.SetSendCount)
		}

		switch v := p.(type) {
		case *srt.DataPacket:
			session.buffer.Add(v)
		}
		conns := session.Connections(1)
		for _, conn := range conns {
			session.sendMeter.Increment(conn.LocalAddr().(*net.UDPAddr))
			if _, err = conn.Write(buf[:n]); err != nil {
				fmt.Printf("error writing pkt %v\n", err)
			}
		}
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {
}
