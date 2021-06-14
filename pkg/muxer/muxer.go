package muxer

import (
	"fmt"
	"net"
	"sync"

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

	w, err := net.DialUDP("udp", nil, dial)
	if err != nil {
		panic(err)
	}

	var msg [1500]byte

	var sourceLock sync.RWMutex
	sources := make(map[uint32]uint32)                  // map from socket id -> connection group
	connectionGroups := make(map[uint32][]*net.UDPAddr) // map from connection group to addresses.

	go func() {
		var p [1500]byte
		for {
			n, err := w.Read(p[:])
			if err != nil {
				fmt.Printf("error reading from destination socket %v\n", err)
				break
			}
			msg, err := srt.Unmarshal(p[:n])
			if err != nil {
				fmt.Printf("not an srt packet %v\n", err)
				continue
			}
			// get the destination socket id for the msg.
			sourceLock.RLock()
			connectionGroup, ok := sources[msg.DestinationSocketId()]
			if !ok {
				fmt.Printf("source socket id does not exist\n")
				continue
			}
			sourceLock.RUnlock()
			// broadcast the message to the entire connection group.
			sources, ok := connectionGroups[connectionGroup]
			if !ok {
				fmt.Printf("connection group does not exist\n")
				continue
			}
			for _, source := range sources {
				if _, err := r.WriteToUDP(msg.Marshal(), source); err != nil {
					fmt.Printf("error writing to source %v\n", source)
				}
			}
		}
	}()

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
			case srt.ControlTypeHandshake:
				// get the connection group for this socket.
				connectionGroup, ok := connectionGroupAssignments[saddr]
				if !ok {
					fmt.Printf("message received from unknown connection group")
					continue READ
				}
				fmt.Printf("binding %d to connection group %d\n", v.HandshakeSocketId(), connectionGroup)
				sourceLock.Lock()
				sources[v.HandshakeSocketId()] = connectionGroup
				sourceLock.Unlock()
				continue READ
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
