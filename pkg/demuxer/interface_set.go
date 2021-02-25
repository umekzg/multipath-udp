package demuxer

import (
	"encoding/hex"
	"fmt"
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
	i.interfaces[getUDPAddrKey(addr)] = addr
	i.Unlock()
}

func (i *InterfaceSet) Remove(addr *net.UDPAddr) {
	fmt.Printf("removing interface %v\n", addr)
	i.Lock()
	delete(i.interfaces, getUDPAddrKey(addr))
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
