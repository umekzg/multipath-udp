package buffer

import (
	"fmt"
	"math"
	"time"

	"github.com/muxfd/multipath-udp/pkg/srt"
)

// ReceiverBuffer represents a linear buffer that discards based on the sequence number.
type ReceiverBuffer struct {
	buffer []*EnqueuedPacket
	count  int
	tail   uint32

	bufferDelay time.Duration

	EmitCh    chan *srt.DataPacket
	MissingCh chan *srt.ControlPacket

	pushCh chan *srt.DataPacket
}

type EnqueuedPacket struct {
	Packet *srt.DataPacket
	EmitAt time.Time
}

func NewReceiverBuffer(bufferDelay time.Duration) *ReceiverBuffer {
	buffer := make([]*EnqueuedPacket, math.MaxUint16)

	r := &ReceiverBuffer{
		buffer:      buffer,
		bufferDelay: bufferDelay,
		EmitCh:      make(chan *srt.DataPacket, math.MaxUint16),
		MissingCh:   make(chan *srt.ControlPacket, math.MaxUint16),
		pushCh:      make(chan *srt.DataPacket, math.MaxUint16),
	}

	go r.runEventLoop()

	return r
}

func (r *ReceiverBuffer) set(seq uint32, p *EnqueuedPacket) {
	r.buffer[int(seq)%len(r.buffer)] = p
}

func (r *ReceiverBuffer) get(seq uint32) *EnqueuedPacket {
	return r.buffer[int(seq)%len(r.buffer)]
}

func (r *ReceiverBuffer) runEventLoop() {
	for {
		// get the packet at the tail.
		if t := r.get(r.tail); t != nil {
			select {
			case p, ok := <-r.pushCh:
				if !ok {
					return
				}
				// add this packet.
				if p.SequenceNumber() < r.tail {
					// packet too old, immediately transmit.
					fmt.Printf("recovered seq %d\n", p.SequenceNumber())
					r.EmitCh <- p
					continue
				}

				// insert this packet into the correct spot taking the latest timestamp.
				if q := r.get(p.SequenceNumber()); q == nil {
					r.set(p.SequenceNumber(), &EnqueuedPacket{
						Packet: p,
						EmitAt: time.Now().Add(r.bufferDelay),
					})
					r.count++
				}
				break
			case <-time.After(time.Until(t.EmitAt)):
				// broadcast the packet at the tail.
				r.EmitCh <- t.Packet
				r.set(r.tail, nil)
				r.tail++
				r.count--
				if r.count > 0 {
					start := r.tail
					for r.get(r.tail) == nil {
						r.tail++
					}
					if r.tail != start {
						fmt.Printf("misisng packets %d - %d (%d)\n", start, r.tail, r.tail-start)
						// write a nak.
						r.MissingCh <- srt.NewNakControlPacket(start, r.tail)
					}
				}
			}
		} else {
			p, ok := <-r.pushCh
			if !ok {
				return
			}
			// this is the first packet.
			r.set(p.SequenceNumber(), &EnqueuedPacket{
				Packet: p,
				EmitAt: time.Now().Add(r.bufferDelay),
			})
			r.tail = p.SequenceNumber()
			r.count++
			continue
		}
	}
}

func (r *ReceiverBuffer) Add(p *srt.DataPacket) {
	r.pushCh <- p
}

func (r *ReceiverBuffer) Close() {
	close(r.pushCh)
	close(r.MissingCh)
	close(r.EmitCh)
}
