package muxer

import (
	"container/ring"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/muxfd/multipath-udp/pkg/protocol"

	"github.com/haivision/srtgo"
)

// Sink represents a multiplexed UDP sink from a single source.
type Sink struct {
	sync.RWMutex

	socket *srtgo.SrtSocket

	head *ring.Ring
	tail *ring.Ring

	length int
}

func NewSink(output *net.UDPAddr) *Sink {
	options := make(map[string]string)
	options["transtype"] = "live"

	socket := srtgo.NewSrtSocket(output.IP.String(), uint16(output.Port), options)
	if err := socket.Connect(); err != nil {
		// TODO: better error handling lol.
		panic(err)
	}

	ring := ring.New(100000)

	sink := &Sink{socket: socket, head: ring, tail: ring, length: 0}

	go sink.writeLoop()

	return sink
}

func (s *Sink) Write(packet protocol.DataPacket) (int, error) {
	s.Lock()
	defer s.Unlock()
	if s.head.Value == nil {
		s.head.Value = packet
	} else {
		// get the last packet's sequence number.
		lastSequence := s.head.Value.(protocol.DataPacket).SequenceNumber
		delta := int(packet.SequenceNumber) - int(lastSequence)
		if delta <= -s.length {
			// lol wtf this packet is so late.
			return 0, errors.New("packet too late")
		}
		s.head.Move(delta).Value = packet
		if delta > 0 {
			s.head = s.head.Move(delta)
		}
	}
	return len(packet.Payload), nil
}

func (s *Sink) writeLoop() {
	for {
		s.Lock()

		clock := time.Now().Add(-2 * time.Second)

		if tail := s.tail.Value; tail != nil {
			packet := tail.(protocol.DataPacket)
			if packet.Timestamp.Before(clock) {
				// ship this packet off.
				if _, err := s.socket.Write(packet.Payload, 100); err != nil {
					fmt.Printf("failed to write packet %v\n", err)
				}
				s.tail.Value = nil
				s.tail = s.tail.Next()
			} else {
				// wait for the packet to be ready.
				time.Sleep(packet.Timestamp.Sub(clock))
			}
		} else if s.tail != s.head {
			// a packet is missing, so skip it.
			s.tail = s.tail.Next()
		} else {
			// waiting for data. just poll, it's good enough.
			time.Sleep(10 * time.Millisecond)
		}

		s.Unlock()
	}
}

func (sink *Sink) Close() {
	sink.socket.Close()
}
