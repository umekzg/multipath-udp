package muxer

import (
	"fmt"
	"net"

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
	r.SetReadBuffer(64 * 1024 * 1024)

	var msg [1500]byte
	sessions := make(map[uint32]*Session)
	connections := make(map[string]uint32)

READ:
	for {
		n, senderAddr, err := r.ReadFromUDP(msg[:])
		if err != nil {
			fmt.Printf("error reading %v\n", err)
			break
		}

		saddr := senderAddr.String()

		p, err := srt.Unmarshal(msg[:n])
		if err != nil {
			fmt.Printf("error unmarshalling rtp packet %v\n", err)
			continue
		}
		switch v := p.(type) {
		case *srt.ControlPacket:
			switch v.ControlType() {
			case srt.ControlTypeUserDefined:
				switch v.Subtype() {
				case srt.SubtypeMultipathHandshake:
					// add it to the corresponding connection group and respond with a handshake.
					id := v.TypeSpecificInformation()
					session, ok := sessions[id]
					if !ok {
						session, err := NewSession(dial)
						if err != nil {
							fmt.Printf("error creating new session %v\n", err)
							continue READ
						}
						sessions[id] = session
						go func() {
							var msg [1500]byte
							for {
								n, err := session.SRTConn.Read(msg[:])
								if err != nil {
									fmt.Printf("error reading %v\n", err)
									break
								}

								for _, sender := range session.GetSenders() {
									if _, err := r.WriteToUDP(msg[:n], sender); err != nil {
										fmt.Printf("error writing response %v\n", err)
										session.RemoveSender(sender)
									}
								}
							}
						}()
						session.AddSender(senderAddr)
						connections[saddr] = id
					} else {
						session.AddSender(senderAddr)
						connections[saddr] = id
					}
					if _, err := r.WriteToUDP(srt.NewMultipathHandshakeControlPacket(id).Marshal(), senderAddr); err != nil {
						fmt.Printf("failed to write handshake response %v\n", err)
					}
					continue READ
				case srt.SubtypeMultipathKeepAlive:
					// ignore the packet.
					continue READ
				}
			}
		}

		id, ok := connections[saddr]
		if !ok {
			fmt.Printf("connection not found, probably missing handshake!\n")
			continue
		}
		session, ok := sessions[id]
		if !ok {
			fmt.Printf("connection not found, probably missing handshake!\n")
			continue
		}
		if _, err := session.SRTConn.Write(p.Marshal()); err != nil {
			fmt.Printf("error forwarding packet to srt %v\n", err)
		}
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (m *Muxer) Close() {
}
