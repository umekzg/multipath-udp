package interfaces

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/muxfd/multipath-udp/pkg/srt"
)

type Connection struct {
	sync.RWMutex
	key    string
	conn   *net.UDPConn
	weight uint32
}

type Container struct {
	sync.RWMutex
	connections []*Connection
	listeners   []chan srt.Packet
	raddr       *net.UDPAddr
}

func NewContainer(raddr *net.UDPAddr) *Container {
	c := &Container{connections: make([]*Connection, 0, 5), raddr: raddr}
	go func() {
		for i := 0; ; i++ {
			time.Sleep(100 * time.Millisecond)
			c.Lock()
			if i%25 == 0 {
				fmt.Printf("--- STATS ---\n")
			}
			for _, conn := range c.connections {
				conn.Lock()
				if i%25 == 0 {
					fmt.Printf("conn\t%v\tweight\t%d\n", conn.key, conn.weight)
				}
				if conn.weight < 1000 {
					conn.weight += 1
				}
				if _, err := conn.conn.Write(srt.NewMultipathKeepAliveControlPacket().Marshal()); err != nil {
					fmt.Printf("connection might have died... %v\n", err)
				}
				conn.Unlock()
			}
			if i%25 == 0 {
				fmt.Printf("---  END  ---\n")
			}
			c.Unlock()
		}
	}()
	return c
}

func getUDPAddrKey(addr *net.UDPAddr) string {
	// handle nil addr differently because it's valid to pass to DialUDP.
	if addr == nil {
		return ""
	}
	return hex.EncodeToString(addr.IP)
}

func (s *Container) Add(addr *net.UDPAddr) error {
	fmt.Printf("adding interface %v\n", addr)
	key := getUDPAddrKey(addr)
	for _, conn := range s.connections {
		if conn.key == key {
			return fmt.Errorf("interface already exists")
		}
	}
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
			pkt, err := srt.Unmarshal(msg[:n])
			if err != nil {
				continue
			}
			s.RLock()
			for _, ch := range s.listeners {
				ch <- pkt
			}
			s.RUnlock()
		}
	}()
	conn := &Connection{
		key:    key,
		conn:   w,
		weight: 1000,
	}
	s.Lock()
	s.connections = append(s.connections, conn)
	s.Unlock()
	return nil
}

func (s *Container) Remove(addr *net.UDPAddr) error {
	fmt.Printf("removing interface %v\n", addr)
	s.Lock()
	defer s.Unlock()
	key := getUDPAddrKey(addr)
	for i, conn := range s.connections {
		if conn.key != key {
			continue
		}
		conn.Lock()
		if err := conn.conn.Close(); err != nil {
			return err
		}
		conn.weight = 0
		conn.Unlock()
		ret := make([]*Connection, len(s.connections)-1)
		copy(ret[:i], s.connections[:i])
		copy(ret[i:], s.connections[i+1:])
		s.connections = ret
		return nil
	}
	return nil
}

func (s *Container) Listen(ch chan srt.Packet) {
	s.Lock()
	defer s.Unlock()
	s.listeners = append(s.listeners, ch)
}

func (s *Container) Unlisten(ch chan srt.Packet) {
	s.Lock()
	defer s.Unlock()
	for i, c := range s.listeners {
		if c == ch {
			ret := make([]chan srt.Packet, len(s.listeners)-1)
			copy(ret[:i], s.listeners[:i])
			copy(ret[i:], s.listeners[i+1:])
			s.listeners = ret
			break
		}
	}
}

func (s *Container) UDPConns() []*net.UDPConn {
	s.RLock()
	defer s.RUnlock()
	result := make([]*net.UDPConn, 0, len(s.connections))
	for _, conn := range s.connections {
		result = append(result, conn.conn)
	}
	return result
}

func (s *Container) ChooseUDPConn() *net.UDPConn {
	s.RLock()
	defer s.RUnlock()
	totalWeights := 0
	for _, conn := range s.connections {
		totalWeights += int(conn.weight)
	}
	choice := rand.Intn(totalWeights)
	cumulative := 0
	for _, conn := range s.connections {
		cumulative += int(conn.weight)
		if cumulative > choice {
			return conn.conn
		}
	}
	return s.connections[len(s.connections)-1].conn
}

func (s *Container) NumConnections() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.connections)
}

func (s *Container) Deduct(senderAddr *net.UDPAddr) {
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

func (s *Container) Close() {
	s.Lock()
	defer s.Unlock()
	for _, conn := range s.connections {
		conn.Lock()
		conn.conn.Close()
		conn.weight = 0
		conn.Unlock()
	}
}
