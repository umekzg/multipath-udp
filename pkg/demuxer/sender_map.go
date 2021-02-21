package demuxer

import (
	"fmt"
	"net"
	"sync"
)

type SenderMap struct {
	sync.RWMutex
	senders map[string]*Sender
}

func NewSenderMap() *SenderMap {
	return &SenderMap{senders: make(map[string]*Sender)}
}

func (s *SenderMap) Get(addr *net.UDPAddr) (*Sender, bool) {
	s.RLock()
	sink, ok := s.senders[addr.String()]
	s.RUnlock()
	return sink, ok
}

func (s *SenderMap) Set(addr *net.UDPAddr, sender *Sender) {
	s.Lock()
	s.senders[addr.String()] = sender
	s.Unlock()
}

func (s *SenderMap) IsEmpty() bool {
	return len(s.senders) == 0
}

func (s *SenderMap) SendAll(msg []byte) {
	s.RLock()
	if len(s.senders) == 0 {
		fmt.Printf("no senders available\n")
	}
	for _, sender := range s.senders {
		sender.send <- msg
	}
	s.RUnlock()
}

func (s *SenderMap) CloseAddr(addr *net.UDPAddr) {
	s.Lock()
	id := addr.String()
	if sender, ok := s.senders[id]; ok {
		sender.Close()
		delete(s.senders, id)
	}
	s.Unlock()
}

func (s *SenderMap) CloseAll() {
	s.Lock()
	for id, sender := range s.senders {
		sender.Close()
		delete(s.senders, id)
	}
	s.Unlock()
}
