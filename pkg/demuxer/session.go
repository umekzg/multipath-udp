package demuxer

import (
	"github.com/muxfd/multipath-udp/pkg/buffer"
	"github.com/muxfd/multipath-udp/pkg/interfaces"
	"github.com/muxfd/multipath-udp/pkg/srt"
)

type Session struct {
	listener       chan srt.Packet
	container      *interfaces.Container
	sourceSocketId uint32
	buffer         *buffer.SenderBuffer
	seq            uint32
}

func NewSession(container *interfaces.Container, sourceSocketId uint32, respCh chan srt.Packet) *Session {
	listener := make(chan srt.Packet, 16)
	go func() {
		for {
			pkt, ok := <-listener
			if !ok {
				break
			}
			if pkt.DestinationSocketId() != sourceSocketId {
				continue
			}
			respCh <- pkt
		}
	}()
	container.Listen(listener)
	return &Session{
		listener:       listener,
		container:      container,
		sourceSocketId: sourceSocketId,
		buffer:         buffer.NewSenderBuffer(),
	}
}

func (s *Session) Close() {
	s.container.Unlisten(s.listener)
	close(s.listener)
}
