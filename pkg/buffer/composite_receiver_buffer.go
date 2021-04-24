package buffer

import (
	"math"
	"time"

	"github.com/muxfd/multipath-udp/pkg/srt"
)

type CompositeReceiverBuffer struct {
	EmitCh    chan srt.Packet
	MissingCh chan *srt.ControlPacket

	pushCh chan srt.Packet

	buffers []*ReceiverBuffer
}

func NewCompositeReceiverBuffer(delays ...time.Duration) *CompositeReceiverBuffer {
	if len(delays) == 0 {
		panic("lmao")
	}

	r := &CompositeReceiverBuffer{
		EmitCh:    make(chan srt.Packet, math.MaxUint16),
		MissingCh: make(chan *srt.ControlPacket, math.MaxUint16),
		pushCh:    make(chan srt.Packet, math.MaxUint16),
		buffers:   make([]*ReceiverBuffer, len(delays)),
	}

	for i, delay := range delays {
		r.buffers[i] = NewReceiverBuffer(delay)
		source := r.pushCh
		if i > 0 {
			source = r.buffers[i-1].EmitCh
		}
		go func(buf *ReceiverBuffer, source chan srt.Packet) {
			for {
				p, ok := <-source
				if !ok {
					buf.Close()
					break
				}
				buf.Add(p)
			}
		}(r.buffers[i], source)
		go func(buf *ReceiverBuffer) {
			for {
				p, ok := <-buf.MissingCh
				if !ok {
					break
				}
				r.MissingCh <- p
			}
		}(r.buffers[i])
	}

	go func(buf *ReceiverBuffer) {
		for {
			p, ok := <-buf.EmitCh
			if !ok {
				break
			}
			r.EmitCh <- p
		}
	}(r.buffers[len(r.buffers)-1])

	return r
}

func (r *CompositeReceiverBuffer) Add(p srt.Packet) {
	r.pushCh <- p
}

func (r *CompositeReceiverBuffer) Close() {
	close(r.EmitCh)
	close(r.pushCh)
}
