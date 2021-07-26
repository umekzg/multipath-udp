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

	container := interfaces.NewContainer(dial)
	d.interfaceBinder.Bind(container.Add, container.Remove, dial)

	responseCh := make(chan srt.Packet, 16)
	container.Listen(responseCh)

	var sourceLock sync.RWMutex
	sources := make(map[uint32]*net.UDPAddr)

	go func() {
		for {
			msg, ok := <-responseCh
			if !ok {
				break
			}
			// get the destination socket id for the msg.
			sourceLock.RLock()
			source, ok := sources[msg.DestinationSocketId()]
			sourceLock.RUnlock()
			if !ok {
				fmt.Printf("unknown response destination socket id %d\n", msg.DestinationSocketId())
				continue
			}
			if _, err := r.WriteToUDP(msg.Marshal(), source); err != nil {
				fmt.Printf("error writing to source %v\n", source)
			}
		}
	}()

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

		switch v := p.(type) {
		case *srt.ControlPacket:
			switch v.ControlType() {
			case srt.ControlTypeHandshake:
				// note this sender addr's socket id.
				sourceLock.Lock()
				fmt.Printf("binding %d to addr %s\n", v.HandshakeSocketId(), senderAddr)
				sources[v.HandshakeSocketId()] = senderAddr
				sourceLock.Unlock()
			}
			// push control packets to all interfaces.
			conns := container.UDPConns()
			if len(conns) == 0 {
				fmt.Printf("no connection to write to\n")
				continue
			}
			for _, conn := range conns {
				if _, err := conn.UDPConn.Write(p.Marshal()); err != nil {
					fmt.Printf("error writing to container %v\n", err)
				}
			}
		case *srt.DataPacket:
			// check if this is a retransmission
			if v.IsRetransmitted() {
				container.MarkFailed(v.SequenceNumber())
			}
			// push data packets to weighted random interface.
			conn := container.ChooseConnection()
			if conn == nil {
				fmt.Printf("no connection to write to\n")
				continue
			}
			if _, err := conn.UDPConn.Write(p.Marshal()); err != nil {
				fmt.Printf("error writing to container %v\n", err)
			}
			if !v.IsRetransmitted() {
				// log the sender.
				container.AssignSender(v.SequenceNumber(), conn)
			}
		}
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {
}
