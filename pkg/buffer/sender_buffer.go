package buffer

import (
	"math"
	"net"

	"github.com/muxfd/multipath-udp/pkg/srt"
)

type Packet struct {
	Sender *net.UDPAddr
	Data   *srt.DataPacket
}

// SenderBuffer represents a linear buffer that discards based on the sequence number.
type SenderBuffer struct {
	buf []*Packet
}

func NewSenderBuffer() *SenderBuffer {
	return &SenderBuffer{buf: make([]*Packet, math.MaxUint16)}
}

func (b *SenderBuffer) Add(sender *net.UDPAddr, data *srt.DataPacket) {
	b.buf[int(data.SequenceNumber())%len(b.buf)] = &Packet{Sender: sender, Data: data}
}

func (b *SenderBuffer) Get(index uint32) *Packet {
	return b.buf[int(index)%len(b.buf)]
}
