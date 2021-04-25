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

type InterfaceSet struct {
	sync.RWMutex
	raddr       *net.UDPAddr
	connections map[string]*net.UDPConn
	buffer      *buffer.SenderBuffer
	responseCh  chan *Message
}

func NewInterfaceSet(raddr *net.UDPAddr, responseCh chan *Message) *InterfaceSet {
	return &InterfaceSet{
		connections: make(map[string]*net.UDPConn),
		raddr:       raddr,
		responseCh:  responseCh,
		buffer:      buffer.NewSenderBuffer()}
}

func getUDPAddrKey(addr *net.UDPAddr) string {
	// handle nil addr differently because it's valid to pass to DialUDP.
	if addr == nil {
		return ""
	}
	return hex.EncodeToString(addr.IP)
}

func (i *InterfaceSet) Add(addr *net.UDPAddr) error {
	fmt.Printf("adding interface %v\n", addr)
	d := &net.Dialer{LocalAddr: addr}
	c, err := d.Dial("udp", i.raddr.String())
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
			i.responseCh <- &Message{msg: msg[:n], addr: w.LocalAddr().(*net.UDPAddr)}
		}
	}()
	i.Lock()
	i.connections[getUDPAddrKey(addr)] = w
	i.Unlock()
	return nil
}

func (i *InterfaceSet) Remove(addr *net.UDPAddr) error {
	fmt.Printf("removing interface %v\n", addr)
	i.Lock()
	defer i.Unlock()
	key := getUDPAddrKey(addr)
	if conn, ok := i.connections[key]; ok {
		if err := conn.Close(); err != nil {
			return err
		}
	}
	delete(i.connections, key)
	return nil
}

func (i *InterfaceSet) Connections() []*net.UDPConn {
	i.RLock()
	conns := make([]*net.UDPConn, 0, len(i.connections))
	for _, conn := range i.connections {
		conns = append(conns, conn)
	}
	i.RUnlock()
	return conns
}

func (i *InterfaceSet) Close() {
	for _, conn := range i.connections {
		conn.Close()
	}
}
