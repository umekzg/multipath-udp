package muxer

import (
	"net"
	"sync"
)

type ReceiverMap struct {
	sync.RWMutex
	receivers map[string]*Receiver
}

func NewReceiverMap() *ReceiverMap {
	return &ReceiverMap{receivers: make(map[string]*Receiver)}
}

func (r *ReceiverMap) Get(addr *net.UDPAddr) (*Receiver, bool) {
	r.RLock()
	sink, ok := r.receivers[addr.String()]
	r.RUnlock()
	return sink, ok
}

func (r *ReceiverMap) Set(addr *net.UDPAddr, receiver *Receiver) {
	r.Lock()
	r.receivers[addr.String()] = receiver
	r.Unlock()
}
