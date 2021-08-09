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
	connectionGroupAssignments := make(map[string]uint32)

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
			// blindly write to all udp sockets in connection group.
			for _, source := range sources {
				pkt := msg.Marshal()
				fmt.Printf("%s <- %d\n", source.String(), len(pkt))
				if _, err := r.WriteToUDP(pkt, source); err != nil {
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

		fmt.Printf("%s -> %d\n", saddr, n)

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
				break
			case srt.ControlTypeUserDefined:
				switch v.Subtype() {
				case srt.SubtypeMultipathHandshake:
					id := v.TypeSpecificInformation()
					// see if this socket has already been assigned.
					if connectionGroup, ok := connectionGroupAssignments[saddr]; ok {
						if connectionGroup != id {
							fmt.Printf("mismatched connection group!\n")
						}
						if _, err := r.WriteToUDP(srt.NewMultipathHandshakeControlPacket(id).Marshal(), senderAddr); err != nil {
							fmt.Printf("failed to write handshake response %v\n", err)
						}
						continue READ
					}
					// add it to the corresponding connection group and respond with a handshake.
					connectionGroupAssignments[saddr] = id
					connectionGroups[id] = append(connectionGroups[id], senderAddr)
					continue READ
				case srt.SubtypeMultipathKeepAlive:
					// ignore the packet.
					continue READ
				}
			}
		}

		if _, err := w.Write(p.Marshal()); err != nil {
			fmt.Printf("error forwarding packet to srt %v\n", err)
		}
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (m *Muxer) Close() {
}
