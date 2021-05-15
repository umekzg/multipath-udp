package demuxer

import (
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

	containers := make(map[string]*interfaces.Container)
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
		container, found := containers[saddr]
		if !found {
			container = interfaces.NewContainer(dial)
			d.interfaceBinder.Bind(container.Add, container.Remove, dial)
			containers[saddr] = container
			responseCh := make(chan srt.Packet, 16)
			container.Listen(responseCh)
			go func() {
				for {
					msg, ok := <-responseCh
					if !ok {
						break
					}
					if _, err := r.WriteToUDP(msg.Marshal(), senderAddr); err != nil {
						fmt.Printf("error writing to source %v\n", senderAddr)
					}
				}
			}()
		}
		conn := container.ChooseUDPConn()
		if conn == nil {
			fmt.Printf("no connection to write to\n")
			continue
		}
		if _, err := conn.Write(p.Marshal()); err != nil {
			fmt.Printf("error writing to container %v\n", err)
		}
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {
}
