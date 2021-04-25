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

	var sourceLock sync.Mutex
	sources := make(map[uint32]*net.UDPAddr)
	interfaces := NewInterfaceSet(dial, d.responseCh)
	if d.interfaceBinder != nil {
		close := d.interfaceBinder.Bind(interfaces.Add, interfaces.Remove, dial)
		defer close()
	}

	rates := make(map[string]uint32)
	var rateLock sync.Mutex

	go func() {
		for {
			msg, ok := <-d.responseCh
			if !ok {
				fmt.Printf("response channel closed\n")
				break
			}
			p, err := srt.Unmarshal(msg.msg)
			if err != nil {
				fmt.Printf("error unmarshaling packet: %v\n", err)
				continue
			}
			switch v := p.(type) {
			case *srt.ControlPacket:
				if v.ControlType() == srt.ControlTypeUserDefined && v.Subtype() == srt.SubtypeMultipathAck {
					rateLock.Lock()
					fmt.Printf("%v %v\n", msg.addr, v.TypeSpecificInformation())
					rates[msg.addr] = v.TypeSpecificInformation()
					rateLock.Unlock()
				}
			}
			sourceLock.Lock()
			// this is kind of weird.
			d := make([]byte, len(msg.msg))
			copy(d, msg.msg)
			if source, ok := sources[p.DestinationSocketId()]; ok {
				if _, err := r.WriteToUDP(d, source); err != nil {
					fmt.Printf("error writing to udp: %v\n", err)
				}
			}
			sourceLock.Unlock()
		}
	}()

	for {
		msg := make([]byte, 1500)

		n, senderAddr, err := r.ReadFromUDP(msg)
		if err != nil {
			fmt.Printf("error reading %v\n", err)
			break
		}

		p, err := srt.Unmarshal(msg[:n])
		if err != nil {
			fmt.Printf("error unmarshaling srt packet %v\n", err)
			return
		}
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
			return
		}

		conns := interfaces.Connections()
		if len(conns) == 0 {
			fmt.Printf("no connections available\n")
			return
		}

		switch v := p.(type) {
		case *srt.DataPacket:
			fmt.Printf("sending pkt %d\n", v.SequenceNumber())
			// write to all interfaces.
			for _, conn := range conns {
				if _, err := conn.Write(msg[:n]); err != nil {
					fmt.Printf("error writing to socket %v: %v\n", conn, err)
				}
			}
		case *srt.ControlPacket:
			// pick a random connection.
			fmt.Printf("control pkt\n")
			conn := conns[0]
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
