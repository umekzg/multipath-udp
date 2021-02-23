package demuxer

import (
	"net"
	"sync"
)

type InterfaceSet struct {
	sync.RWMutex
	interfaces map[string]*net.UDPAddr
}

func NewInterfaceSet() *InterfaceSet {
	return &InterfaceSet{interfaces: make(map[string]*net.UDPAddr)}
}

func getKey(addr *net.UDPAddr) string {
	// handle nil addr differently because it's valid to pass to DialUDP.
	if addr == nil {
		return ""
	}
	return addr.String()
}

func (i *InterfaceSet) Add(addr *net.UDPAddr) {
	i.Lock()
	i.interfaces[getKey(addr)] = addr
	i.Unlock()
}

func (i *InterfaceSet) Remove(addr *net.UDPAddr) {
	i.Lock()
	delete(i.interfaces, getKey(addr))
	i.Unlock()
}

func (s *InterfaceSet) GetAll() []*net.UDPAddr {
	s.RLock()
	var addrs []*net.UDPAddr
	for _, addr := range s.interfaces {
		addrs = append(addrs, addr)
	}
	s.RUnlock()
	return addrs
}
