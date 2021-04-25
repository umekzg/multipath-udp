package demuxer

import (
	"fmt"
	"net"

	"github.com/muxfd/multipath-udp/pkg/interfaces"
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

	sessions := make(map[string]*InterfaceSet)

	respCh := make(chan *Message, 128)

	var buffer [1500]byte
	for {
		n, senderAddr, err := r.ReadFromUDP(buffer[0:])
		if err != nil {
			break
		}
		saddr := senderAddr.String()
		session, found := sessions[saddr]
		if !found {
			session = NewInterfaceSet(dial, respCh)
			close := d.interfaceBinder.Bind(session.Add, session.Remove, dial)
			defer close()
			defer session.Close()

			sessions[saddr] = session

			go func() {
				for {
					resp, ok := <-respCh
					if !ok {
						break
					}
					if _, err = r.WriteToUDP(resp.msg, senderAddr); err != nil {
						fmt.Printf("error writing response %v\n", err)
						break
					}
				}
			}()
		}
		conns := session.Connections()
		for _, conn := range conns {
			if _, err = conn.Write(buffer[0:n]); err != nil {
				fmt.Printf("error writing pkt %v\n", err)
			}
		}
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {
}
