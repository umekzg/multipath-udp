package demuxer

import (
	"encoding/hex"
	"fmt"
	"net"
	"sync"

	"github.com/muxfd/multipath-udp/pkg/buffer"
)

type Message struct {
	addr *net.UDPAddr
	msg  []byte
}

type Session struct {
	sync.RWMutex
	raddr       *net.UDPAddr
	connections map[string]*net.UDPConn
	buffer      *buffer.SenderBuffer
	responseCh  chan *Message
	negotiated  bool
}

func NewSession(raddr *net.UDPAddr, responseCh chan *Message) *Session {
	return &Session{
		connections: make(map[string]*net.UDPConn),
		raddr:       raddr,
		responseCh:  responseCh,
		buffer:      buffer.NewSenderBuffer(),
		negotiated:  false,
	}
}

func getUDPAddrKey(addr *net.UDPAddr) string {
	// handle nil addr differently because it's valid to pass to DialUDP.
	if addr == nil {
		return ""
	}
	return hex.EncodeToString(addr.IP)
}

func (s *Session) Add(addr *net.UDPAddr) error {
	fmt.Printf("adding interface %v\n", addr)
	d := &net.Dialer{LocalAddr: addr}
	c, err := d.Dial("udp", s.raddr.String())
	if err != nil {
		return err
	}
	w := c.(*net.UDPConn)
	go func() {
		for {
			var msg [1500]byte
			n, err := w.Read(msg[0:])
			if err != nil {
				break
			}
			s.responseCh <- &Message{msg: msg[:n], addr: w.LocalAddr().(*net.UDPAddr)}
		}
	}()
	s.Lock()
	s.connections[getUDPAddrKey(addr)] = w
	s.Unlock()
	return nil
}

func (s *Session) Remove(addr *net.UDPAddr) error {
	fmt.Printf("removing interface %v\n", addr)
	s.Lock()
	defer s.Unlock()
	key := getUDPAddrKey(addr)
	if conn, ok := s.connections[key]; ok {
		if err := conn.Close(); err != nil {
			return err
		}
	}
	delete(s.connections, key)
	return nil
}

func (s *Session) Connections() []*net.UDPConn {
	s.RLock()
	conns := make([]*net.UDPConn, 0, len(s.connections))
	for _, conn := range s.connections {
		conns = append(conns, conn)
	}
	s.RUnlock()
	return conns
}

func (s *Session) IsNegotiated() bool {
	s.RLock()
	defer s.RUnlock()
	return s.negotiated
}

func (s *Session) SetNegotiated(val bool) {
	s.Lock()
	defer s.Unlock()
	s.negotiated = val
}

func (s *Session) Close() {
	for _, conn := range s.connections {
		conn.Close()
	}
}
