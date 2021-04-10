package demuxer

import (
	"fmt"
	"math/rand"
	"net"
	"sync"

	"github.com/muxfd/multipath-udp/pkg/interfaces"
	"github.com/muxfd/multipath-udp/pkg/srt"
)

// Demuxer represents a UDP stream demuxer that demuxes a source over multiple senders.
type Demuxer struct {
	responseCh chan []byte

	interfaceBinder *interfaces.AutoBinder
}

// NewDemuxer creates a new demuxer.
func NewDemuxer(options ...func(*Demuxer)) *Demuxer {
	d := &Demuxer{responseCh: make(chan []byte, 1024)}

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
	r.SetReadBuffer(1024 * 1024)
	r.SetWriteBuffer(1024 * 1024)

	var sourceLock sync.Mutex
	sources := make(map[uint32]*net.UDPAddr)
	interfaces := NewInterfaceSet(dial, d.responseCh)
	if d.interfaceBinder != nil {
		close := d.interfaceBinder.Bind(interfaces.Add, interfaces.Remove, dial)
		defer close()
	}

	go func() {
		for {
			msg, ok := <-d.responseCh
			if !ok {
				fmt.Printf("response channel closed\n")
				break
			}
			p, err := srt.Unmarshal(msg)
			if err != nil {
				fmt.Printf("error unmarshaling packet: %v\n", err)
				continue
			}
			sourceLock.Lock()
			if source, ok := sources[p.DestinationSocketId()]; ok {
				if _, err := r.WriteToUDP(msg, source); err != nil {
					fmt.Printf("error writing to udp: %v\n", err)
				}
			}
			sourceLock.Unlock()
		}
	}()

	for {
		msg := make([]byte, 2048)

		n, senderAddr, err := r.ReadFromUDP(msg)
		if err != nil {
			fmt.Printf("error reading %v\n", err)
			break
		}

		p, err := srt.Unmarshal(msg[:n])
		if p.DestinationSocketId() == 0 {
			switch v := p.(type) {
			case *srt.ControlPacket:
				sourceLock.Lock()
				sources[v.HandshakeSocketId()] = senderAddr
				sourceLock.Unlock()
			}
		}
		if err != nil {
			fmt.Printf("error unmarshalling rtp packet %v\n", err)
			continue
		}
		conns := interfaces.Connections()
		switch p.(type) {
		case *srt.DataPacket:
			if len(conns) <= 2 {
				// write to all interfaces.
				for _, conn := range conns {
					if _, err := conn.Write(msg[:n]); err != nil {
						fmt.Printf("error writing to socket %v: %v\n", conn, err)
					}
				}
			} else {
				// pick two.
				a := rand.Intn(len(conns))
				if _, err := conns[a].Write(msg[:n]); err != nil {
					fmt.Printf("error writing to socket %v: %v\n", conns[a], err)
				}
				if _, err := conns[(a+1)%len(conns)].Write(msg[:n]); err != nil {
					fmt.Printf("error writing to socket %v: %v\n", conns[(a+1)%len(conns)], err)
				}
			}
		case *srt.ControlPacket:
			// pick a random connection.
			i := rand.Intn(len(conns))
			for _, conn := range conns {
				if i == 0 {
					if _, err := conn.Write(msg[:n]); err != nil {
						fmt.Printf("error writing to socket %v: %v\n", conn, err)
					}
					break
				} else {
					i--
				}
			}
		}
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {
	close(d.responseCh)
}
