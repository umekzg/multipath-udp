package muxer

import (
	"net"
	"time"

	"github.com/muxfd/multipath-udp/pkg/buffer"
)

type Session struct {
	SRTConn *net.UDPConn
	buffer  *buffer.CompositeReceiverBuffer
	sources []*net.UDPAddr
}

func NewSession(dial *net.UDPAddr) (*Session, error) {
	conn, err := net.DialUDP("udp", nil, dial)
	if err != nil {
		return nil, err
	}

	return &Session{
		SRTConn: conn,
		buffer:  buffer.NewCompositeReceiverBuffer(200*time.Millisecond, 200*time.Millisecond, 200*time.Millisecond),
		sources: make([]*net.UDPAddr, 0, 5),
	}, nil
}

func (s *Session) AddSender(senderAddr *net.UDPAddr) {
	for _, addr := range s.sources {
		if addr.Port == senderAddr.Port {
			return
		}
	}
	s.sources = append(s.sources, senderAddr)
}
