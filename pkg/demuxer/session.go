package demuxer

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/muxfd/multipath-udp/pkg/buffer"
)

type Message struct {
	addr *net.UDPAddr
	msg  []byte
}

type Connection struct {
	sync.RWMutex
	key     string
	conn    *net.UDPConn
	weight  uint32
	deleted bool
}

type Session struct {
	sync.RWMutex
	raddr       *net.UDPAddr
	connections []*Connection

	buffer     *buffer.SenderBuffer
	responseCh chan *Message
	negotiated bool
}

func NewSession(raddr *net.UDPAddr, responseCh chan *Message) *Session {
	return &Session{
		connections: make([]*Connection, 0, 5),
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
	conn := &Connection{
		key:     getUDPAddrKey(addr),
		conn:    w,
		deleted: false,
		weight:  60,
	}
	go func() {
		for {
			time.Sleep(1 * time.Second)
			conn.Lock()
			if conn.deleted {
				break
			}
			fmt.Printf("conn\t%v\tweight\t%d\n", conn.key, conn.weight)
			if conn.weight < 60 {
				conn.weight += 1
			}
			conn.Unlock()
		}
	}()
	s.Lock()
	s.connections = append(s.connections, conn)
	s.Unlock()
	return nil
}

func (s *Session) Remove(addr *net.UDPAddr) error {
	fmt.Printf("removing interface %v\n", addr)
	s.Lock()
	defer s.Unlock()
	key := getUDPAddrKey(addr)
	for _, conn := range s.connections {
		if !conn.deleted && conn.key == key {
			conn.Lock()
			conn.deleted = true
			if err := conn.conn.Close(); err != nil {
				return err
			}
			conn.Unlock()
		}
	}
	return nil
}

func (s *Session) Connections() []*net.UDPConn {
	s.RLock()
	result := make([]*net.UDPConn, 0, len(s.connections))
	for _, conn := range s.connections {
		if !conn.deleted {
			result = append(result, conn.conn)
		}
	}
	s.RUnlock()
	return result
}

func (s *Session) ChooseConnection() *net.UDPConn {
	totalWeights := 0
	for _, conn := range s.connections {
		if !conn.deleted {
			totalWeights += int(conn.weight)
		}
	}
	choice := rand.Intn(totalWeights)
	cumulative := 0
	for _, conn := range s.connections {
		if conn.deleted {
			continue
		}
		cumulative += int(conn.weight)
		if cumulative > choice {
			return conn.conn
		}
	}
	return s.connections[len(s.connections)-1].conn
}

func (s *Session) NumConnections() int {
	return len(s.connections)
}

func (s *Session) Deduct(senderAddr *net.UDPAddr) {
	s.Lock()
	defer s.Unlock()
	for _, conn := range s.connections {
		if conn.conn.LocalAddr().(*net.UDPAddr).String() == senderAddr.String() {
			conn.Lock()
			if conn.weight > 1 {
				conn.weight--
			}
			conn.Unlock()
		}
	}
}

func (s *Session) Close() {
	for _, conn := range s.connections {
		conn.conn.Close()
	}
}
