package demuxer

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"sync"

	"github.com/muxfd/multipath-udp/pkg/interfaces"
	"github.com/muxfd/multipath-udp/pkg/srt"
	"gonum.org/v1/gonum/stat/sampleuv"
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
	r.SetReadBuffer(1024 * 1024)
	r.SetWriteBuffer(1024 * 1024)

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
		msg := make([]byte, 2048)

		n, senderAddr, err := r.ReadFromUDP(msg)
		if err != nil {
			fmt.Printf("error reading %v\n", err)
			break
		}

		p, err := srt.Unmarshal(msg[:n])
		if err != nil {
			fmt.Printf("error unmarshaling srt packet %v\n", err)
			continue
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
			continue
		}
		conns := interfaces.Connections()
		if len(conns) == 0 {
			fmt.Printf("no connections available\n")
			continue
		}
		switch p.(type) {
		case *srt.DataPacket:
			if len(conns) <= 3 {
				// write to all interfaces.
				for _, conn := range conns {
					if _, err := conn.Write(msg[:n]); err != nil {
						fmt.Printf("error writing to socket %v: %v\n", conn, err)
					}
				}
			} else {
				// pick two.
				w := make([]float64, len(conns))
				rateLock.Lock()
				for i, c := range conns {
					if x, ok := rates[c.LocalAddr().String()]; ok {
						w[i] = math.Log2(math.Max(float64(x), 2))
					} else {
						w[i] = 1
					}
				}
				rateLock.Unlock()
				rng := sampleuv.NewWeighted(w, nil)
				a, ok := rng.Take()
				if !ok {
					fmt.Printf("error sampling weights %v", w)
				}
				b, ok := rng.Take()
				if !ok {
					fmt.Printf("error sampling weights %v", w)
				}
				// fmt.Printf("using %v %v\n", w, rates)
				if _, err := conns[a].Write(msg[:n]); err != nil {
					fmt.Printf("error writing to socket %v: %v\n", conns[a], err)
				}
				if _, err := conns[b].Write(msg[:n]); err != nil {
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
