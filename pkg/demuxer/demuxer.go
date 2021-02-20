package demuxer

import (
	"crypto/rand"
	"fmt"
	"net"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/segmentio/fasthash/fnv1a"
)

// Demuxer represents a UDP stream demuxer that demuxes a source over multiple senders.
type Demuxer struct {
	sources map[string]*Source

	interfaces map[*net.UDPAddr]bool
	output     *net.UDPAddr

	handshakeTimeout time.Duration

	quit chan bool
	done *sync.WaitGroup
}

// NewDemuxer creates a new demuxer.
func NewDemuxer(listen, dial *net.UDPAddr, options ...func(*Demuxer)) *Demuxer {
	var wg sync.WaitGroup
	d := &Demuxer{
		sources:          make(map[string]*Source),
		interfaces:       make(map[*net.UDPAddr]bool),
		output:           dial,
		handshakeTimeout: 1 * time.Second,
		quit:             make(chan bool),
		done:             &wg,
	}

	for _, option := range options {
		option(d)
	}

	ready := make(chan bool)
	wg.Add(1)
	go func() {
		inputConn, err := net.ListenUDP("udp", listen)
		if err != nil {
			panic(err)
		}
		defer inputConn.Close()
		defer wg.Done()

		wg.Add(1)
		go func() {
			defer wg.Done()
			ready <- true
			if <-d.quit {
				fmt.Printf("quit signal received for muxer\n")
				inputConn.Close()
			}
		}()

		for {
			msg := make([]byte, 2048)

			n, senderAddr, err := inputConn.ReadFromUDP(msg)
			if err != nil {
				fmt.Printf("input conn read failed %v: %v\n", inputConn, err)
				break
			}

			d.GetSource(inputConn, dial, senderAddr).ReceiveMessage(msg[:n])
		}
	}()
	<-ready
	return d
}

// Wait waits for the demuxer to exit.
func (d *Demuxer) Wait() {
	d.done.Wait()
}

// AddInterface adds a given local address networking interface
func (d *Demuxer) AddInterface(laddr *net.UDPAddr) {
	fmt.Printf("adding interface %v\n", laddr)

	// add the addr to the set of interfaces.
	d.interfaces[laddr] = true

	// add it to all existing sources.
	for _, source := range d.sources {
		if sender, ok := source.senders[laddr]; ok {
			fmt.Printf("source already exists for addr %s: %v\n", laddr, sender)
		} else {
			source.AddSender(laddr, d.output)
		}
	}
}

// RemoveInterface removes a given local address networking interface
func (d *Demuxer) RemoveInterface(laddr *net.UDPAddr) {
	fmt.Printf("removing interface %v\n", laddr)

	delete(d.interfaces, laddr)

	// remove it from all existing sources.
	for _, source := range d.sources {
		if sender, ok := source.senders[laddr]; ok {
			sender.Close()
			delete(source.senders, laddr)
		}
	}
}

// GetSource returns the source for a given output address and message source.
func (d *Demuxer) GetSource(input *net.UDPConn, output *net.UDPAddr, clientAddr *net.UDPAddr) *Source {
	if source, ok := d.sources[clientAddr.String()]; ok {
		return source
	}
	send := make(chan []byte, 2048)
	recv := make(chan []byte, 2048)
	token := make([]byte, 64)
	_, err := rand.Read(token)
	if err != nil {
		fmt.Printf("failed to generate random bytes: %v\n", err)
	}
	source := &Source{
		ID:               token,
		address:          clientAddr,
		senders:          make(map[*net.UDPAddr]*Sender),
		handshakeTimeout: d.handshakeTimeout,
		send:             send,
		recv:             recv,
	}
	d.sources[clientAddr.String()] = source

	// bind existing interfaces
	for laddr := range d.interfaces {
		source.AddSender(laddr, output)
	}

	// msg read loop
	go func() {
		for {
			msg, ok := <-source.recv
			if !ok {
				fmt.Printf("read loop terminated for sender %v\n", source)
				break
			}

			if len(source.senders) == 0 {
				fmt.Printf("no senders available for message %s\n", msg)
			} else {
				for _, sender := range source.senders {
					sender.send <- msg
				}
			}
		}
	}()

	// msg write loop
	go func() {
		cache, err := lru.New(10000000)
		if err != nil {
			fmt.Printf("failed to create cache for source %v\n", source)
			return
		}
		for {
			msg, ok := <-source.send
			if !ok {
				fmt.Printf("write loop terminated for source %v\n", source)
				break
			}
			if found, _ := cache.ContainsOrAdd(fnv1a.HashBytes64(msg), true); found {
				continue
			}
			_, err := input.WriteToUDP(msg, clientAddr)
			if err != nil {
				fmt.Printf("error writing response message address %v: %v\n", source, err)
			}
		}
	}()

	return source
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {
	for _, source := range d.sources {
		source.Close()
	}
	d.quit <- true
	d.done.Wait()
}
