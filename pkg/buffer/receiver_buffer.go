package buffer

import (
	"math"
	"sync"
	"time"

	"github.com/pion/rtp"
)

// ReceiverBuffer represents a linear buffer that discards based on the sequence number.
type ReceiverBuffer struct {
	sync.RWMutex

	buffer []*rtp.Packet
	count  int
	tail   uint16

	bufferDelay          time.Duration
	retransmissionDelays []time.Duration

	clockRate    uint32
	timestamp0   uint32
	clock0       time.Time
	minTimestamp uint32

	EmitCh chan *rtp.Packet
	LossCh chan *LossReport

	pushCh chan *rtp.Packet
}

type LossReport struct {
	SequenceNumber uint16
	Count          uint16
}

func NewReceiverBuffer(clockRate uint32, bufferDelay time.Duration, retransmissionDelays ...time.Duration) *ReceiverBuffer {
	// each packet is ~1.5kb, assume an inbound max rate of 60 Mbps => 40k packets/sec
	buffer := make([]*rtp.Packet, math.MaxUint16)

	r := &ReceiverBuffer{
		buffer: buffer, tail: 0,
		bufferDelay:          bufferDelay,
		retransmissionDelays: retransmissionDelays,
		clockRate:            clockRate,
		EmitCh:               make(chan *rtp.Packet, math.MaxUint16),
		LossCh:               make(chan *LossReport, math.MaxUint16),
		pushCh:               make(chan *rtp.Packet, math.MaxUint16),
	}

	go r.runEventLoop()

	return r
}

func (r *ReceiverBuffer) runEventLoop() {
	for {
		// get the packet at the tail.
		if t := r.buffer[r.tail]; t != nil {
			dtimestamp := t.Timestamp - r.timestamp0
			dt := time.Duration(dtimestamp/r.clockRate) * time.Second
			remaining := r.clock0.Add(dt).Add(r.bufferDelay).Sub(time.Now())
			select {
			case p := <-r.pushCh:
				// add this packet.
				if p.Timestamp <= r.minTimestamp {
					// packet too old.
					continue
				}

				// insert this packet into the correct spot taking the latest timestamp.
				if q := r.buffer[p.SequenceNumber]; q == nil {
					r.buffer[p.SequenceNumber] = p
					r.count++
					// progress the head if this packet within r.head + 65536/4
				}
				break
			case <-time.After(remaining):
				// broadcast the packet at the tail.
				r.minTimestamp = t.Timestamp
				r.EmitCh <- t
				r.buffer[r.tail] = nil
				r.tail++
				r.count--
				if r.count > 0 {
					for r.buffer[r.tail] == nil {
						r.tail++
					}
				}
			}

		} else {
			p := <-r.pushCh
			// this is the first packet.
			r.buffer[p.SequenceNumber] = p
			r.timestamp0 = p.Timestamp
			r.clock0 = time.Now()
			r.tail = p.SequenceNumber
			r.count++
			continue
		}
	}
}

func (r *ReceiverBuffer) Add(p *rtp.Packet) {
	r.pushCh <- p
}
