package muxer

import (
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/muxfd/multipath-udp/pkg/buffer"
	"github.com/muxfd/multipath-udp/pkg/srt"
)

type Muxer struct {
	buf *buffer.ReceiverBuffer

	responseCh chan []byte
}

// NewMuxer creates a new uniplex listener muxer
func NewMuxer(options ...func(*Muxer)) *Muxer {
	buf := buffer.NewReceiverBuffer(2 * time.Second)
	m := &Muxer{buf: buf, responseCh: make(chan []byte, 128)}

	for _, option := range options {
		option(m)
	}

	return m
}

func (m *Muxer) Start(listen, dial *net.UDPAddr) {
	go m.readLoop(listen)
	go m.writeLoop(dial)
}

func (m *Muxer) writeLoop(dial *net.UDPAddr) {
	w, err := net.DialUDP("udp", nil, dial)
	if err != nil {
		panic(err)
	}
	w.SetReadBuffer(1024 * 1024)
	w.SetWriteBuffer(1024 * 1024)

	go func() {
		for {
			msg := make([]byte, 2048)
			n, err := w.Read(msg)
			if err != nil {
				fmt.Printf("failed to read response %v\n", err)
				break
			}

			p, err := srt.Unmarshal(msg[:n])
			if err != nil {
				fmt.Printf("failed to unmarshal response %v\n", err)
				continue
			}

			if !p.IsControl {
				// send all data packets upstream.
				m.responseCh <- p.Marshal()
				continue
			}

			c, err := p.ToControlPacket()
			if err != nil {
				fmt.Printf("failed to convert response %v\n", err)
				continue
			}

			if c.ControlType != srt.ControlTypeNak {
				// only send non-nak's upstream.
				m.responseCh <- p.Marshal()
			}
		}
	}()

	for {
		p, ok := <-m.buf.EmitCh
		if !ok {
			break
		}
		fmt.Printf("seq: %d\n", p.SequenceNumber)
		if _, err := w.Write(p.Marshal()); err != nil {
			fmt.Printf("failed to write packet %v\n", err)
		}
	}
}

func (m *Muxer) readLoop(listen *net.UDPAddr) {
	r, err := net.ListenUDP("udp", listen)
	if err != nil {
		panic(err)
	}
	r.SetReadBuffer(1024 * 1024)
	r.SetWriteBuffer(1024 * 1024)

	senders := make(map[string]*net.UDPAddr)
	var senderLock sync.Mutex

	go func() {
		for {
			msg, ok := <-m.responseCh
			if !ok {
				break
			}

			// broadcast to all senders since this is a control packet.
			senderLock.Lock()
			for _, sender := range senders {
				r.WriteToUDP(msg, sender)
			}
			senderLock.Unlock()
		}
	}()

	for {
		msg := make([]byte, 2048)

		n, senderAddr, err := r.ReadFromUDP(msg)
		if err != nil {
			fmt.Printf("error reading %v\n", err)
			break
		}

		senderLock.Lock()
		senders[hex.EncodeToString(senderAddr.IP.To16())] = senderAddr
		senderLock.Unlock()

		p, err := srt.Unmarshal(msg[:n])
		if err != nil {
			fmt.Printf("error unmarshalling rtp packet %v\n", err)
			continue
		}

		m.buf.Add(p)
	}
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (m *Muxer) Close() {
	m.buf.Close()
	close(m.responseCh)
}
