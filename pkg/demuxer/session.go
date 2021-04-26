package demuxer

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/muxfd/multipath-udp/pkg/buffer"
	"github.com/muxfd/multipath-udp/pkg/meter"
)

type Message struct {
	addr *net.UDPAddr
	msg  []byte
}

type Connection struct {
	key     string
	conn    *net.UDPConn
	deleted bool
}

type Session struct {
	sync.RWMutex
	raddr       *net.UDPAddr
	connections []*Connection

	sendMeter *meter.Meter
	sendCt    map[string]uint32
	recvCt    map[string]uint32

	buffer     *buffer.SenderBuffer
	responseCh chan *Message
	negotiated bool
}

func NewSession(raddr *net.UDPAddr, responseCh chan *Message) *Session {
	return &Session{
		connections: make([]*Connection, 0, 5),
		sendMeter:   meter.NewMeter(1 * time.Second),
		sendCt:      make(map[string]uint32),
		recvCt:      make(map[string]uint32),
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
	s.connections = append(s.connections, &Connection{
		key:     getUDPAddrKey(addr),
		conn:    w,
		deleted: false,
	})
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
			if err := conn.conn.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Session) SetRecvCount(addr *net.UDPAddr, val uint32) {
	s.Lock()
	defer s.Unlock()
	s.recvCt[addr.String()] = val
}

func (s *Session) SetSendCount(addr *net.UDPAddr, val uint32) {
	s.Lock()
	defer s.Unlock()
	s.sendCt[addr.String()] = val
}

func (s *Session) getPacketsInFlight(addr *net.UDPAddr) uint32 {
	send := s.sendCt[addr.String()]
	recv := s.recvCt[addr.String()]
	if recv > send {
		// just a time desync
		return 0
	}
	return send - recv // number of packets in flight, approx.
}

func (s *Session) Connections(count int) []*net.UDPConn {
	if count <= 0 {
		return make([]*net.UDPConn, 0)
	}
	s.RLock()
	if count < s.NumConnections() {
		sort.Slice(s.connections, func(i, j int) bool {
			a := s.connections[i].conn.LocalAddr().(*net.UDPAddr)
			b := s.connections[j].conn.LocalAddr().(*net.UDPAddr)
			return s.getPacketsInFlight(a) < s.getPacketsInFlight(b)
		})
	}
	if rand.Float64() < 0.001 {
		for _, conn := range s.connections {
			addr := conn.conn.LocalAddr().(*net.UDPAddr)
			fmt.Printf("conn %v pkts in flight %d (%d, %d)\n",
				conn.conn,
				s.getPacketsInFlight(addr),
				s.sendCt[addr.String()],
				s.recvCt[addr.String()])
		}
	}
	result := make([]*net.UDPConn, 0, count)
	for i, conn := range s.connections {
		if i >= count {
			break
		}
		result = append(result, conn.conn)
	}
	s.RUnlock()
	return result
}

func (s *Session) NumConnections() int {
	return len(s.connections)
}

func (s *Session) Close() {
	for _, conn := range s.connections {
		conn.conn.Close()
	}
}
