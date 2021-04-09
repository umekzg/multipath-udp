package buffer

import (
	"container/ring"
	"errors"
)

// SenderBuffer represents a linear buffer that discards based on the sequence number.
type SenderBuffer struct {
	ring        *ring.Ring
	size        int
	activeIndex int
}

func NewSenderBuffer(n int) *SenderBuffer {
	return &SenderBuffer{ring: ring.New(n), size: n}
}

func (b *SenderBuffer) Add(data interface{}) {
	b.ring.Value = data
	b.ring = b.ring.Next()
	b.activeIndex++
}

func (b *SenderBuffer) Get(index int) (interface{}, error) {
	if index >= b.activeIndex {
		return nil, errors.New("index out of range: too large")
	} else if index < 0 {
		return nil, errors.New("index out of range: negative")
	}
	delta := index - b.activeIndex
	if delta < -b.size {
		return nil, errors.New("index out of range: too small, discarded")
	}
	return b.ring.Move(delta).Value, nil
}
