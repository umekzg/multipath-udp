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

	sessions := make(map[string]*Session)
	container := interfaces.NewContainer(dial)
	d.interfaceBinder.Bind(container.Add, container.Remove, dial)

REQUEST:
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
			switch v := p.(type) {
			case *srt.ControlPacket:
				if v.ControlType() != srt.ControlTypeHandshake {
					fmt.Printf("non-handshake packet sent first, sending shutdown.")
					continue REQUEST
				}
			case *srt.DataPacket:
				fmt.Printf("non-handshake packet sent first, sending shutdown.")
				continue REQUEST
			}
			respCh := make(chan srt.Packet, 16)
			session = NewSession(container, p.(*srt.ControlPacket).HandshakeSocketId(), respCh)
			sessions[saddr] = session

			go func() {
				nakTimestamps := make(map[uint32]bool)
			RESPONSE:
				for {
					p, ok := <-respCh
					if !ok {
						fmt.Printf("response ch closed\n")
						break
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
									continue RESPONSE
								}
								nakTimestamps[timestamp] = true
								severity := v.TypeSpecificInformation()
								from := binary.BigEndian.Uint32(v.RawPacket[16:20])
								to := binary.BigEndian.Uint32(v.RawPacket[20:24])
								fmt.Printf("nak %d-%d severity %d\n", from, to, severity)
								for i := from; i < to; i++ {
									pkt := session.buffer.Get(i)
									if pkt == nil {
										if severity == 0 {
											fmt.Printf("failed to fulfill nak %d in time, packet didn't exist\n", i)
										}
										if _, err := r.WriteToUDP(
											srt.NewNakSingleControlPacket(session.sourceSocketId, i).Marshal(),
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
									session.container.Deduct(pkt.Sender)
									for _, conn := range session.container.UDPConns() {
										if _, err = conn.Write(pkt.Data.Marshal()); err != nil {
											fmt.Printf("error writing pkt %v\n", err)
										}
									}
								}
							}
						case srt.ControlTypeShutdown:
							fmt.Printf("------------------\n")
							fmt.Printf("SHUTDOWN ATTEMPT\n")
							fmt.Printf("------------------\n")
							delete(sessions, saddr)
							session.Close()
							if n, err := r.WriteToUDP(p.Marshal(), senderAddr); err != nil || n != len(p.Marshal()) {
								fmt.Printf("error writing response %v\n", err)
								continue RESPONSE
							}
						case srt.ControlTypeHandshake:
							fmt.Printf("handshake %d <- %d\n", v.DestinationSocketId(), v.HandshakeSocketId())
							fallthrough
						default:
							if n, err := r.WriteToUDP(p.Marshal(), senderAddr); err != nil || n != len(p.Marshal()) {
								fmt.Printf("error writing response %v\n", err)
								continue RESPONSE
							}
						}
					case *srt.DataPacket:
						if _, err = r.WriteToUDP(p.Marshal(), senderAddr); err != nil {
							fmt.Printf("error writing response %v\n", err)
							continue RESPONSE
						}
					}
				}
			}()
		}

		switch v := p.(type) {
		case *srt.DataPacket:
			conn := session.container.ChooseUDPConn()
			addr := conn.LocalAddr().(*net.UDPAddr)
			if v.SequenceNumber() > session.seq {
				if session.seq > 0 && v.SequenceNumber() > session.seq+1 {
					// emit nak immediately, localhost -> localhost is usually reliably ordered
					// and if it's not it's cheap to send so whatever.
					fmt.Printf("short circuit nak %d-%d (%d)\n", session.seq+1, v.SequenceNumber()-1, v.SequenceNumber()-session.seq-1)
					if _, err := r.WriteToUDP(
						srt.NewNakRangeControlPacket(session.sourceSocketId, session.seq+1, v.SequenceNumber()-1).Marshal(),
						senderAddr,
					); err != nil {
						fmt.Printf("error writing short nak\n")
					}
				}
				// might be an out of order retransmission.
				session.seq = v.SequenceNumber()
			}
			session.buffer.Add(addr, v)
			if _, err = conn.Write(buf[:n]); err != nil {
				fmt.Printf("error writing pkt %v\n", err)
				session.container.Deduct(conn.LocalAddr().(*net.UDPAddr))
			}
		case *srt.ControlPacket:
			switch v.ControlType() {
			case srt.ControlTypeHandshake:
				session.sourceSocketId = v.HandshakeSocketId()
				fmt.Printf("handshake %d -> %d\n", v.HandshakeSocketId(), v.DestinationSocketId())
			case srt.ControlTypeShutdown:
				fmt.Printf("------------------\n")
				fmt.Printf("SHUTDOWN ATTEMPT\n")
				fmt.Printf("------------------\n")
				delete(sessions, saddr)
				session.Close()
			}
			for _, conn := range session.container.UDPConns() {
				if _, err = conn.Write(buf[:n]); err != nil {
					fmt.Printf("error writing pkt %v\n", err)
					session.container.Deduct(conn.LocalAddr().(*net.UDPAddr))
				}
			}
		}
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {
}
