package muxer

import (
	"net"
	"sync"
)

type Session struct {
	sync.RWMutex

	SRTConn *net.UDPConn
	sources []*net.UDPAddr
}

func NewSession(dial *net.UDPAddr) (*Session, error) {
	conn, err := net.DialUDP("udp", nil, dial)
	if err != nil {
		return nil, err
	}

	return &Session{
		SRTConn: conn,
		sources: make([]*net.UDPAddr, 0, 5),
	}, nil
}

func (s *Session) AddSender(senderAddr *net.UDPAddr) {
	s.Lock()
	defer s.Unlock()

	for _, addr := range s.sources {
		if addr.String() == senderAddr.String() {
			return
		}
	}
	s.sources = append(s.sources, senderAddr)
}

func (s *Session) GetSenders() []*net.UDPAddr {
	s.RLock()
	defer s.RUnlock()

	addrs := make([]*net.UDPAddr, len(s.sources))
	copy(addrs, s.sources)
	return addrs
}

func (s *Session) RemoveSender(senderAddr *net.UDPAddr) {
	s.Lock()
	defer s.Unlock()

	for i, addr := range s.sources {
		if addr.String() == senderAddr.String() {
			ret := make([]*net.UDPAddr, len(s.sources)-1)
			copy(ret[:i], s.sources[:i])
			copy(ret[i:], s.sources[i+1:])
			s.sources = ret
			return
		}
	}
}
