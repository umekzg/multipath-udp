package muxer

import "sync"

type SinkMap struct {
	sync.RWMutex
	sinks map[uint64]*Sink
}

func NewSinkMap() *SinkMap {
	return &SinkMap{sinks: make(map[uint64]*Sink)}
}

func (s *SinkMap) Get(id uint64) (*Sink, bool) {
	s.RLock()
	sink, ok := s.sinks[id]
	s.RUnlock()
	return sink, ok
}

func (s *SinkMap) Set(id uint64, sink *Sink) {
	s.Lock()
	s.sinks[id] = sink
	s.Unlock()
}

func (s *SinkMap) CloseAll() {
	s.Lock()
	for id, sink := range s.sinks {
		sink.Close()
		delete(s.sinks, id)
	}
	s.Unlock()
}
