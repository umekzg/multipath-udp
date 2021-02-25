package demuxer

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/muxfd/multipath-udp/pkg/deduplicator"
	"github.com/muxfd/multipath-udp/pkg/protocol"
	"github.com/muxfd/multipath-udp/pkg/scheduler"
)

// Demuxer represents a UDP stream demuxer that demuxes a source over multiple senders.
type Demuxer struct {
	senders  map[string]*Sender
	sessions map[string][]byte

	deduplicator deduplicator.Deduplicator
	scheduler    scheduler.Scheduler

	conn *net.UDPConn

	interfaces *InterfaceSet

	handshakeTimeout time.Duration

	done *sync.WaitGroup
}

// NewDemuxer creates a new demuxer.
func NewDemuxer(listen, dial *net.UDPAddr, options ...func(*Demuxer)) *Demuxer {
	conn, err := net.ListenUDP("udp", listen)
	if err != nil {
		panic(err)
	}
	conn.SetReadBuffer(1024 * 1024)
	conn.SetWriteBuffer(1024 * 1024)
	var wg sync.WaitGroup
	d := &Demuxer{
		senders:          make(map[string]*Sender),
		sessions:         make(map[string][]byte),
		conn:             conn,
		interfaces:       NewInterfaceSet(),
		handshakeTimeout: 1 * time.Second,
		done:             &wg,
	}

	wg.Add(1)

	for _, option := range options {
		option(d)
	}

	if d.scheduler != nil {
		d.scheduler.SenderInit(dial)
	}

	go d.readLoop(dial)

	return d
}

func (d *Demuxer) readLoop(dial *net.UDPAddr) {
	defer d.done.Done()
	for {
		msg := make([]byte, 2048)
		n, senderAddr, err := d.conn.ReadFromUDP(msg)
		if err != nil {
			fmt.Printf("error reading %v\n", err)
			break
		}

		session := d.getSession(senderAddr)

		ifaces := d.interfaces.GetAll()
		if d.scheduler != nil {
			ifaceIDMap := make(map[string]*net.UDPAddr)
			ifaceIDs := make([]string, len(ifaces))
			for i, iface := range ifaces {
				key := getUDPAddrKey(iface)
				ifaceIDMap[key] = iface
				ifaceIDs[i] = key
			}
			var filtered []*net.UDPAddr
			for _, ifaceID := range d.scheduler.Schedule(ifaceIDs, msg) {
				iface, ok := ifaceIDMap[ifaceID]
				if !ok {
					fmt.Printf("invalid interface id %s scheduled\n", ifaceID)
					continue
				}
				filtered = append(filtered, iface)
			}
			ifaces = filtered
		}

		for _, iface := range ifaces {
			ifaceID := getUDPAddrKey(iface)
			handshake, err := protocol.NewHandshake(ifaceID, session).Serialize()
			if err != nil {
				fmt.Printf("error serializing handshake for ifaceID %s session %s: %v", ifaceID, session, err)
				continue
			}
			key := hex.EncodeToString(handshake)
			sender, ok := d.senders[key]
			if !ok {
				fmt.Printf("new sender over %v with handshake %v\n", iface, handshake)
				sender = NewSender(handshake, iface, dial, func(msg []byte) {
					if d.deduplicator == nil || d.deduplicator.FromReceiver(msg) {
						d.conn.WriteToUDP(msg, senderAddr)
					}
				}, d.handshakeTimeout)
				d.senders[key] = sender
			}
			sender.Write(msg[:n])
		}
	}
}

func (d *Demuxer) getSession(addr *net.UDPAddr) []byte {
	if session, ok := d.sessions[addr.String()]; ok {
		return session
	}
	token := make([]byte, 64)
	_, err := rand.Read(token)
	if err != nil {
		fmt.Printf("failed to generate random bytes: %v\n", err)
	}
	d.sessions[addr.String()] = token
	return token
}

// Wait waits for the demuxer to exit.
func (d *Demuxer) Wait() {
	d.done.Wait()
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {
	d.conn.Close()
	for _, sender := range d.senders {
		sender.Close()
	}
}
