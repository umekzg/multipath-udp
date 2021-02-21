package demuxer

import (
	"net"
	"sync"
)

type InterfaceSet struct {
	sync.RWMutex
	interfaces map[*net.UDPAddr]bool
}

func NewInterfaceSet() *InterfaceSet {
	return &InterfaceSet{interfaces: make(map[*net.UDPAddr]bool)}
}

func (i *InterfaceSet) Add(addr *net.UDPAddr) {
	i.Lock()
	i.interfaces[addr] = true
	i.Unlock()
}

func (i *InterfaceSet) Remove(addr *net.UDPAddr) {
	i.Lock()
	delete(i.interfaces, addr)
	i.Unlock()
}

func (s *InterfaceSet) GetAll() []*net.UDPAddr {
	s.RLock()
	var addrs []*net.UDPAddr
	for addr := range s.interfaces {
		addrs = append(addrs, addr)
	}
	s.RUnlock()
	return addrs
}
