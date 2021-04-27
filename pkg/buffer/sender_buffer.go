package buffer

import (
	"fmt"
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
	seq uint32 // used for metrics.
}

func NewSenderBuffer() *SenderBuffer {
	return &SenderBuffer{buf: make([]*Packet, 1<<24), seq: 0}
}

func (b *SenderBuffer) Add(sender *net.UDPAddr, data *srt.DataPacket) {
	if b.seq != 0 && b.seq+1 != data.SequenceNumber() {
		fmt.Printf("sequence number jump expect %d got %d (diff %d)\n", b.seq+1, data.SequenceNumber(), data.SequenceNumber()-b.seq-1)
	}
	b.seq = data.SequenceNumber()
	b.buf[int(data.SequenceNumber())%len(b.buf)] = &Packet{Sender: sender, Data: data}
}

func (b *SenderBuffer) Get(index uint32) *Packet {
	return b.buf[int(index)%len(b.buf)]
}
