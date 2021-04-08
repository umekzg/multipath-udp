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
	head   int
	mid    int
	tail   int

	bufferDelay         time.Duration
	retransmissionDelay time.Duration

	clockRate  uint32
	timestamp0 uint32
	clock0     time.Time

	EmitCh chan *rtp.Packet
	LossCh chan *LossReport
}

type LossReport struct {
	SequenceNumber uint16
	Count          uint16
}

func NewReceiverBuffer(clockRate uint32, bufferDelay, retransmissionDelay time.Duration) *ReceiverBuffer {
	// each packet is ~1.5kb, assume an inbound max rate of 60 Mbps => 40k packets/sec
	buffer := make([]*rtp.Packet, math.MaxUint16)
	return &ReceiverBuffer{
		buffer: buffer, head: 0, mid: 0, tail: 0,
		bufferDelay:         bufferDelay,
		retransmissionDelay: retransmissionDelay,
		clockRate:           clockRate,
		EmitCh:              make(chan *rtp.Packet, math.MaxUint16),
		LossCh:              make(chan *LossReport, math.MaxUint16),
	}
}

func (r *ReceiverBuffer) get(index int) *rtp.Packet {
	return r.buffer[index%len(r.buffer)]
}

func (r *ReceiverBuffer) set(index int, p *rtp.Packet) {
	r.buffer[index%len(r.buffer)] = p
}

func (r *ReceiverBuffer) isEmpty() bool {
	return r.get(r.head) == nil
}

func (r *ReceiverBuffer) runTailLoop() {
	for {
		r.Lock()
		if r.isEmpty() {
			// the buffer is empty, stop the goroutine.
			r.Unlock()
			return
		}
		// get the packet at r.tail.
		p := r.get(r.tail)
		// compute the expected emission time.
		dtimestamp := p.Timestamp - r.timestamp0
		dt := time.Duration(dtimestamp/r.clockRate) * time.Second
		remaining := r.clock0.Add(dt).Add(r.bufferDelay).Sub(time.Now())
		// wait until it's supposed to be written.
		time.Sleep(remaining)
		// emit the packet.
		r.EmitCh <- p
		// move the tail up if necessary.
		r.set(r.tail, nil)
		if r.tail < r.head {
			r.tail++
		}

		r.Unlock()
	}
}

func (r *ReceiverBuffer) runMidLoop() {
	for {
		r.Lock()
		if r.isEmpty() {
			// the buffer is empty, stop the goroutine.
			r.Unlock()
			return
		} else if r.mid == r.head {
			// this might end up in an awkward state if the head hasn't read any packets.
			// so throw in a sleep here to let the tail catch up.
			time.Sleep(5 * time.Millisecond)
			r.Unlock()
			continue
		}
		// get the packet at r.tail.
		p := r.get(r.mid)
		// compute the expected emission time.
		dtimestamp := p.Timestamp - r.timestamp0
		dt := time.Duration(dtimestamp/r.clockRate) * time.Second
		remaining := r.clock0.Add(dt).Add(r.retransmissionDelay).Sub(time.Now())
		// wait until it's supposed to be written.
		time.Sleep(remaining)
		// move the mid up if necessary, notifying lost packets.
		if r.mid < r.head {
			r.mid++
			count := uint16(0)
			for r.get(r.mid) == nil {
				// this packet is missing.
				count++
				r.mid++
			}
			if count > 0 {
				r.LossCh <- &LossReport{
					SequenceNumber: p.SequenceNumber + 1,
					Count:          count,
				}
			}
		}

		r.Unlock()
	}

}

func (r *ReceiverBuffer) Add(p *rtp.Packet) {
	r.Lock()
	defer r.Unlock()

	h := r.get(r.head)
	t := r.get(r.tail)

	if h == nil {
		// this is the first packet, so insert it and start the eviction listeners.
		r.set(r.head, p)
		r.timestamp0 = p.Timestamp
		r.clock0 = time.Now()

		go r.runTailLoop()
		go r.runMidLoop()

		return
	}

	complement := (h.SequenceNumber + math.MaxUint16/2) % math.MaxUint16
	projectedSequence := int(p.SequenceNumber)

	if p.SequenceNumber < complement {
		projectedSequence += math.MaxUint16
	}

	if h.SequenceNumber < p.SequenceNumber {
		// this packet is in the future. advance the head.
		delta := projectedSequence - int(h.SequenceNumber)
		r.head += delta
		r.set(r.head, p)
	} else if t.SequenceNumber <= p.SequenceNumber {
		// insert this packet in the correct spot, taking the latest timestamp.
		delta := projectedSequence - int(h.SequenceNumber)
		if q := r.get(r.head + delta); q == nil || q.Timestamp < p.Timestamp {
			r.set(r.head+delta, p)
		}
	} else {
		// this packet is too old, ignore it. maybe log a warning.
	}
}
