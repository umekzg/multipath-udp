package muxer

import (
	"fmt"
	"net"
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"github.com/segmentio/fasthash/fnv1a"
)

type Muxer struct {
	receivers *ReceiverMap
	sinks     *SinkMap

	quit chan bool
	done *sync.WaitGroup
}

// NewMuxer creates a new multiplexed listener muxer
func NewMuxer(listen, dial *net.UDPAddr, options ...func(*Muxer)) *Muxer {
	var wg sync.WaitGroup
	m := &Muxer{
		receivers: NewReceiverMap(),
		sinks:     NewSinkMap(),
		quit:      make(chan bool),
		done:      &wg,
	}
	wg.Add(1)

	for _, option := range options {
		option(m)
	}

	ready := make(chan bool)
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
			if <-m.quit {
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

			m.GetReceiver(inputConn, dial, senderAddr).ReceiveResponse(msg[:n])
		}
	}()
	<-ready
	return m
}

// Wait for the muxer to terminate.
func (m *Muxer) Wait() {
	m.done.Wait()
}

// GetSink fetches a sink for a specific output address and id.
func (m *Muxer) GetSink(output *net.UDPAddr, sinkID uint64) *Sink {
	if sink, ok := m.sinks.Get(sinkID); ok {
		return sink
	}
	send := make(chan []byte, 2048)
	cache, err := lru.New(10000000)
	if err != nil {
		panic(err)
	}
	conn, err := net.DialUDP("udp", nil, output)
	if err != nil {
		panic(err)
	}
	sink := &Sink{
		id:        sinkID,
		receivers: NewReceiverSet(),
		conn:      conn,
		cache:     cache,
		send:      send,
	}
	m.sinks.Set(sinkID, sink)

	// read inbound channel
	go func() {
		for {
			msg := make([]byte, 2048)
			n, sender, err := conn.ReadFromUDP(msg)
			if err != nil {
				fmt.Printf("read error from udp address %s: %v\n", sender, err)
				break
			}
			sink.receivers.Send(msg[:n])
		}
	}()

	// pipe write channel
	go func() {
		for {
			msg, ok := <-send
			if !ok {
				fmt.Printf("write loop closed for sink %d\n", sink.id)
				break
			}
			_, err := conn.Write(msg)
			if err != nil {
				fmt.Printf("failed to write msg %s for sink %d\n", msg, sink.id)
			}
		}
	}()
	return sink
}

func (m *Muxer) GetReceiver(input *net.UDPConn, output *net.UDPAddr, source *net.UDPAddr) *Receiver {
	if sender, ok := m.receivers.Get(source); ok {
		return sender
	}
	fmt.Printf("new receiver from addr %v\n", source)
	send := make(chan []byte, 2048)
	recv := make(chan []byte, 2048)
	receiver := &Receiver{
		address: source,
		send:    send,
		recv:    recv,
	}
	m.receivers.Set(source, receiver)

	// msg read loop
	go func() {
		var sink *Sink
		for {
			msg, ok := <-receiver.recv
			if !ok {
				fmt.Printf("read loop terminated for sender %s\n", source)
				break
			}

			hash := fnv1a.HashBytes64(msg)
			if sink == nil || sink.id == hash {
				// get the sink for the incoming sink id
				sink = m.GetSink(output, hash)
				sink.receivers.Add(receiver)
				// respond with the sink id
				_, err := input.WriteToUDP(msg, source)
				if err != nil {
					fmt.Printf("failed to respond to sender %s: %v\n", source, err)
				}
			} else {
				_, err := sink.ReceiveRequest(msg)
				if err != nil {
					fmt.Printf("failed to forward message to sink for sender %s: %v\n", source, err)
				}
			}
		}
	}()

	// msg write loop
	go func() {
		for {
			msg, ok := <-receiver.send
			if !ok {
				fmt.Printf("write loop terminated for sender %s\n", source)
				break
			}
			_, err := input.WriteToUDP(msg, source)
			if err != nil {
				fmt.Printf("error writing response message address %s: %v\n", source, err)
			}
		}
	}()

	return receiver
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (m *Muxer) Close() {
	m.sinks.CloseAll()
	m.quit <- true
	m.Wait()
}
