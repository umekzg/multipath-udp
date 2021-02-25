package muxer

import (
	"bytes"
	"net"
	"sync"

	"github.com/muxfd/multipath-udp/pkg/protocol"
)

type SenderLogEntry struct {
	addr       *net.UDPAddr
	handshake  *protocol.Handshake
	serialized []byte
}

type SenderLog struct {
	sync.RWMutex
	// TODO: there is a memory leak here as this records all historical senders,
	// but doesn't prune off senders that have timed out. we need to determine what
	// is an appropriate timeout for UDP sockets to no longer send, as the sender
	// needs to know that it has to renegotiate.
	//
	// srt sends explicit keep alive packets, perhaps we should also send a signalling
	// packet or something to probe the socket.
	senders map[string]*SenderLogEntry
}

func NewSenderLog() *SenderLog {
	return &SenderLog{senders: make(map[string]*SenderLogEntry)}
}

func (s *SenderLog) GetLogEntry(addr *net.UDPAddr) (*SenderLogEntry, bool) {
	s.RLock()
	defer s.RUnlock()
	sender, ok := s.senders[addr.String()]
	return sender, ok
}

func (s *SenderLog) GetUDPAddrs(session []byte) []*net.UDPAddr {
	s.RLock()
	defer s.RUnlock()
	var addrs []*net.UDPAddr
	for _, sender := range s.senders {
		if bytes.Equal(session, sender.handshake.Session) {
			addrs = append(addrs, sender.addr)
		}
	}
	return addrs
}

func (s *SenderLog) Set(addr *net.UDPAddr, handshake *protocol.Handshake, serialized []byte) {
	s.senders[addr.String()] = &SenderLogEntry{addr: addr, handshake: handshake, serialized: serialized}
}

func (s *SenderLog) Delete(addr *net.UDPAddr) {
	delete(s.senders, addr.String())
}
