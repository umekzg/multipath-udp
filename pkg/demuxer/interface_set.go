package demuxer

import (
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"
)

type InterfaceSet struct {
	sync.RWMutex
	sessionID string
	raddr     *net.UDPAddr
	senders   map[string]*Sender
}

func NewInterfaceSet(sessionID string, raddr *net.UDPAddr) *InterfaceSet {
	return &InterfaceSet{sessionID: sessionID, senders: make(map[string]*Sender), raddr: raddr}
}

func getUDPAddrKey(addr *net.UDPAddr) string {
	// handle nil addr differently because it's valid to pass to DialUDP.
	if addr == nil {
		return ""
	}
	return hex.EncodeToString(addr.IP)
}

func (i *InterfaceSet) Add(addr *net.UDPAddr) {
	fmt.Printf("adding interface %v\n", addr)
	i.Lock()
	i.senders[getUDPAddrKey(addr)] = NewSender(i.sessionID, addr, i.raddr, 3*time.Second)
	i.Unlock()
}

func (i *InterfaceSet) Remove(addr *net.UDPAddr) {
	fmt.Printf("removing interface %v\n", addr)
	i.Lock()
	key := getUDPAddrKey(addr)
	if sender, ok := i.senders[key]; ok {
		sender.Close()
	}
	delete(i.senders, key)
	i.Unlock()
}

func (s *InterfaceSet) Senders() []*Sender {
	s.RLock()
	var senders []*Sender
	for _, sender := range s.senders {
		senders = append(senders, sender)
	}
	s.RUnlock()
	return senders
}
