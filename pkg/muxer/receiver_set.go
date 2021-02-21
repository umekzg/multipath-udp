package muxer

import (
	"sync"
)

type ReceiverSet struct {
	sync.RWMutex
	receivers map[*Receiver]bool
}

func NewReceiverSet() *ReceiverSet {
	return &ReceiverSet{receivers: make(map[*Receiver]bool)}
}

func (r *ReceiverSet) Add(receiver *Receiver) {
	r.Lock()
	r.receivers[receiver] = true
	r.Unlock()
}

func (r *ReceiverSet) Remove(receiver *Receiver) {
	r.Lock()
	delete(r.receivers, receiver)
	r.Unlock()
}

func (r *ReceiverSet) Send(msg []byte) {
	r.RLock()
	for receiver := range r.receivers {
		receiver.send <- msg
	}
	r.RUnlock()
}

func (r *ReceiverSet) CloseAll() {
	r.Lock()
	for receiver := range r.receivers {
		receiver.Close()
		delete(r.receivers, receiver)
	}
	r.Unlock()
}
