package muxer

import (
	"bytes"
	"net"
)

type AddrToSessionMap struct {
	sessions   map[string][]byte
	serialized map[string]*net.UDPAddr
}

func NewAddrToSessionMap() *AddrToSessionMap {
	return &AddrToSessionMap{
		sessions:   make(map[string][]byte),
		serialized: make(map[string]*net.UDPAddr),
	}
}

func (m *AddrToSessionMap) GetSession(addr *net.UDPAddr) ([]byte, bool) {
	session, ok := m.sessions[addr.String()]
	return session, ok
}

func (m *AddrToSessionMap) GetUDPAddrs(session []byte) []*net.UDPAddr {
	var addrs []*net.UDPAddr
	for serialized, s := range m.sessions {
		if bytes.Equal(session, s) {
			addrs = append(addrs, m.serialized[serialized])
		}
	}
	return addrs
}

func (m *AddrToSessionMap) Set(addr *net.UDPAddr, session []byte) {
	m.serialized[addr.String()] = addr
	m.sessions[addr.String()] = session
}

func (m *AddrToSessionMap) Delete(addr *net.UDPAddr) {
	delete(m.sessions, addr.String())
	delete(m.serialized, addr.String())
}
