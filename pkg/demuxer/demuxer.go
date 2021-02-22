package demuxer

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/segmentio/fasthash/fnv1a"
)

// Demuxer represents a UDP stream demuxer that demuxes a source over multiple senders.
type Demuxer struct {
	senders  map[string]*Sender
	sessions map[string][]byte

	caches map[string]*lru.Cache

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

	go func() {
		defer wg.Done()
		if err != nil {
			panic(err)
		}
		for {
			msg := make([]byte, 2048)

			n, senderAddr, err := conn.ReadFromUDP(msg)
			if err != nil {
				fmt.Printf("input conn read failed %v: %v\n", conn, err)
				break
			}

			session := d.GetSession(senderAddr)

			for _, iface := range d.interfaces.GetAll() {
				key := fmt.Sprintf("%s-%s-%s", hex.EncodeToString(session), iface, dial)
				sender, ok := d.senders[key]
				if !ok {
					sender = NewSender(session, iface, dial, func(msg []byte) {
						if !d.IsDuplicateMessage(hex.EncodeToString(session), msg[:n]) {
							conn.WriteToUDP(msg, senderAddr)
						}
					}, d.handshakeTimeout)
					d.senders[key] = sender
				}
				sender.Write(msg[:n])
			}
		}
	}()

	return d
}

func (d *Demuxer) GetSession(addr *net.UDPAddr) []byte {
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

func (d *Demuxer) IsDuplicateMessage(session string, msg []byte) bool {
	cache, ok := d.caches[session]
	if !ok {
		cache, err := lru.New(10000000)
		if err != nil {
			panic(err)
		}
		d.caches[session] = cache
	}
	found, _ := cache.ContainsOrAdd(fnv1a.HashBytes64(msg), true)
	return found
}

// Wait waits for the demuxer to exit.
func (d *Demuxer) Wait() {
	d.done.Wait()
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {
	for _, sender := range d.senders {
		sender.Close()
	}
	d.conn.Close()
}
