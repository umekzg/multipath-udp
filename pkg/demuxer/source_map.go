package demuxer

import (
	"fmt"
	"net"
	"sync"
)

type SourceMap struct {
	sync.RWMutex
	sources map[string]*Source
}

func NewSourceMap() *SourceMap {
	return &SourceMap{sources: make(map[string]*Source)}
}

func (s *SourceMap) Get(addr *net.UDPAddr) (*Source, bool) {
	s.RLock()
	sink, ok := s.sources[addr.String()]
	s.RUnlock()
	return sink, ok
}

func (s *SourceMap) Set(addr *net.UDPAddr, source *Source) {
	s.Lock()
	s.sources[addr.String()] = source
	s.Unlock()
}

func (s *SourceMap) IsEmpty() bool {
	s.RLock()
	defer s.RUnlock()
	return len(s.sources) == 0
}

func (s *SourceMap) SendAll(msg []byte) {
	s.RLock()
	for _, sender := range s.sources {
		sender.send <- msg
	}
	s.RUnlock()
}

func (s *SourceMap) AddSender(addr *net.UDPAddr, output *net.UDPAddr) {
	s.RLock()
	for _, source := range s.sources {
		if sender, ok := source.senders.Get(addr); ok {
			fmt.Printf("source already exists for addr %s: %v\n", addr, sender)
		} else {
			source.AddSender(addr, output)
		}
	}
	s.RUnlock()
}

func (s *SourceMap) CloseAddr(addr *net.UDPAddr) {
	s.Lock()
	for _, source := range s.sources {
		source.senders.CloseAddr(addr)
	}
	s.Unlock()
}

func (s *SourceMap) CloseAll() {
	s.Lock()
	for id, sender := range s.sources {
		sender.Close()
		delete(s.sources, id)
	}
	s.Unlock()
}
