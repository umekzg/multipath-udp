package buffer

import (
	"math"

	"github.com/muxfd/multipath-udp/pkg/srt"
)

// SenderBuffer represents a linear buffer that discards based on the sequence number.
type SenderBuffer struct {
	buf []*srt.DataPacket
}

func NewSenderBuffer() *SenderBuffer {
	return &SenderBuffer{buf: make([]*srt.DataPacket, math.MaxUint16)}
}

func (b *SenderBuffer) Add(pkt *srt.DataPacket) {
	b.buf[int(pkt.SequenceNumber())%len(b.buf)] = pkt
}

func (b *SenderBuffer) Get(index uint32) *srt.DataPacket {
	return b.buf[int(index)%len(b.buf)]
}
