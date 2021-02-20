package muxer

import (
	"fmt"
	"net"

	lru "github.com/hashicorp/golang-lru"
	"github.com/segmentio/fasthash/fnv1a"
)

// Receiver represents an inbound UDP socket bound to a sink.
type Receiver struct {
	address *net.UDPAddr
	send    chan []byte
	recv    chan []byte // unprocessed messages from the source.
}

// Sink represents a multiplexed UDP sink from a single source.
type Sink struct {
	id        uint64
	receivers map[*Receiver]bool
	conn      *net.UDPConn
	cache     *lru.Cache
	send      chan []byte
}

func (sink *Sink) AddSender(receiver *Receiver) {
	sink.receivers[receiver] = true
}

func (sink *Sink) RemoveSender(receiver *Receiver) {
	delete(sink.receivers, receiver)
}

func (sink *Sink) ReceiveRequest(b []byte) (int, error) {
	if found, _ := sink.cache.ContainsOrAdd(fnv1a.HashBytes64(b), true); found {
		return 0, fmt.Errorf("duplicate message")
	}
	return sink.conn.Write(b)
}

func (sink *Sink) Close() {
	for receiver := range sink.receivers {
		receiver.Close()
	}
	sink.conn.Close()
	close(sink.send)
}

func (receiver *Receiver) ReceiveResponse(msg []byte) {
	receiver.recv <- msg
}

func (receiver *Receiver) Close() {
	close(receiver.recv)
	close(receiver.send)
}
