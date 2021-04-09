package demuxer

import (
	"fmt"
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
		// write to all interfaces.
		for _, conn := range interfaces.Connections() {
			if _, err := conn.Write(msg[:n]); err != nil {
				fmt.Printf("error writing to socket %v: %v\n", conn, err)
			}
		}
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {
	close(d.responseCh)
}
