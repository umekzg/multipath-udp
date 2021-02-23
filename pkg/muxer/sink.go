package muxer

import (
	"net"
)

// Sink represents a multiplexed UDP sink from a single source.
type Sink struct {
	conn *net.UDPConn
}

func NewSink(output *net.UDPAddr, onReceive func([]byte)) *Sink {
	conn, err := net.DialUDP("udp", nil, output)
	if err != nil {
		panic(err)
	}
	conn.SetReadBuffer(1024 * 1024)
	conn.SetWriteBuffer(1024 * 1024)
	sink := &Sink{conn: conn}

	// read inbound channel
	go func() {
		for {
			msg := make([]byte, 2048)
			n, _, err := conn.ReadFromUDP(msg)
			if err != nil {
				break
			}
			onReceive(msg[:n])
		}
	}()

	return sink
}

func (sink *Sink) Write(b []byte) (int, error) {
	return sink.conn.Write(b)
}

func (sink *Sink) Close() {
	sink.conn.Close()
}
