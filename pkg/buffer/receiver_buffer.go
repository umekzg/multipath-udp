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
	head   uint32
	tail   uint32

	bufferDelay          time.Duration
	retransmissionDelays []time.Duration

	EmitCh chan *srt.Packet
	LossCh chan *LossReport

	pushCh chan *srt.Packet
}

type LossReport struct {
	SequenceNumber uint16
	Count          uint16
}

type EnqueuedPacket struct {
	Packet *srt.Packet
	EmitAt time.Time
}

func NewReceiverBuffer(bufferDelay time.Duration, retransmissionDelays ...time.Duration) *ReceiverBuffer {
	buffer := make([]*EnqueuedPacket, math.MaxUint16)

	r := &ReceiverBuffer{
		buffer:               buffer,
		bufferDelay:          bufferDelay,
		retransmissionDelays: retransmissionDelays,
		EmitCh:               make(chan *srt.Packet, math.MaxUint16),
		LossCh:               make(chan *LossReport, math.MaxUint16),
		pushCh:               make(chan *srt.Packet, math.MaxUint16),
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
			remaining := time.Until(t.EmitAt)
			select {
			case p, ok := <-r.pushCh:
				if !ok {
					return
				}
				// add this packet.
				if p.SequenceNumber <= t.Packet.SequenceNumber {
					// packet too old.
					continue
				}

				// insert this packet into the correct spot taking the latest timestamp.
				if q := r.get(p.SequenceNumber); q == nil {
					fmt.Printf("adding seq %d\n", p.SequenceNumber)
					r.set(p.SequenceNumber, &EnqueuedPacket{
						Packet: p,
						EmitAt: time.Now().Add(r.bufferDelay),
					})
					r.count++
					// progress the head if this packet within r.head + 65536/4
				}
				break
			case <-time.After(remaining):
				// broadcast the packet at the tail.
				fmt.Printf("tail seq: %d\n", t.Packet.SequenceNumber)
				r.EmitCh <- t.Packet
				r.set(r.tail, nil)
				r.tail++
				r.count--
				if r.count > 0 {
					for r.get(r.tail) == nil {
						r.tail++
					}
				}
			}

		} else {
			p, ok := <-r.pushCh
			if !ok {
				return
			}
			// this is the first packet.
			fmt.Printf("adding seq %d\n", p.SequenceNumber)
			r.set(p.SequenceNumber, &EnqueuedPacket{
				Packet: p,
				EmitAt: time.Now().Add(r.bufferDelay),
			})
			r.tail = p.SequenceNumber
			r.count++
			continue
		}
	}
}

func (r *ReceiverBuffer) Add(p *srt.Packet) {
	if p.IsControl {
		// forward control packets immediately
		r.EmitCh <- p
	} else {
		r.pushCh <- p
	}
}

func (r *ReceiverBuffer) Close() {
	close(r.pushCh)
	close(r.EmitCh)
	close(r.LossCh)
}
