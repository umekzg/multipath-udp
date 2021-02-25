package muxer

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"sync"

	"github.com/muxfd/multipath-udp/pkg/deduplicator"
	"github.com/muxfd/multipath-udp/pkg/protocol"
	"github.com/muxfd/multipath-udp/pkg/scheduler"
)

type Muxer struct {
	senderLog *SenderLog
	sinks     map[string]*Sink

	deduplicator deduplicator.Deduplicator
	scheduler    scheduler.Scheduler // the scheduler is here for telemetry data.

	conn *net.UDPConn
	done *sync.WaitGroup
}

// NewMuxer creates a new multiplexed listener muxer
func NewMuxer(listen, dial *net.UDPAddr, options ...func(*Muxer)) *Muxer {
	conn, err := net.ListenUDP("udp", listen)
	if err != nil {
		panic(err)
	}
	conn.SetReadBuffer(1024 * 1024)
	conn.SetWriteBuffer(1024 * 1024)
	var wg sync.WaitGroup
	m := &Muxer{
		senderLog: NewSenderLog(),
		sinks:     make(map[string]*Sink),
		conn:      conn,
		done:      &wg,
	}
	wg.Add(1)

	for _, option := range options {
		option(m)
	}

	if m.scheduler != nil {
		m.scheduler.ReceiverInit(listen.Port)
	}

	go m.readLoop(dial)

	return m
}

func (m *Muxer) readLoop(dial *net.UDPAddr) {
	defer m.done.Done()

	for {
		msg := make([]byte, 2048)

		n, senderAddr, err := m.conn.ReadFromUDP(msg)
		if err != nil {
			fmt.Printf("error reading %v\n", err)
			break
		}

		log, ok := m.senderLog.GetLogEntry(senderAddr)
		if !ok {
			handshake, err := protocol.Deserialize(msg[:n])
			if err != nil {
				fmt.Printf("first message from socket is not a valid handshake: %v\n", err)
				continue
			}
			fmt.Printf("new session from %v with handshake %v\n", senderAddr, msg[:n])
			m.senderLog.Set(senderAddr, handshake, msg[:n])
			if _, err = m.conn.WriteToUDP(msg[:n], senderAddr); err != nil {
				fmt.Printf("error writing handshake response %v", err)
			}
			continue
		} else if bytes.Equal(log.serialized, msg[:n]) {
			// duplicate handshake, respond until it's successful.
			fmt.Printf("duplicate handshake received\n")
			if _, err := m.conn.WriteToUDP(msg[:n], senderAddr); err != nil {
				fmt.Printf("error writing handshake response %v", err)
			}
			continue
		}

		// forward this message to the sink for the session.
		key := hex.EncodeToString(log.handshake.Session[:])
		if m.deduplicator == nil || m.deduplicator.FromSender(msg) {
			sink, ok := m.sinks[key]
			if !ok {
				sink = NewSink(dial, func(msg []byte) {
					// forward this message to all senders with the same session id as this one.
					for _, sender := range m.senderLog.GetUDPAddrs(log.handshake.Session[:]) {
						m.conn.WriteToUDP(msg, sender)
					}
				})
				m.sinks[key] = sink
			}
			sink.Write(msg[:n])
		}
		if m.scheduler != nil {
			go m.scheduler.Receive(log.handshake.Sender, msg[:n])
		}
	}
}

// Wait for the muxer to terminate.
func (m *Muxer) Wait() {
	m.done.Wait()
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (m *Muxer) Close() {
	m.conn.Close()
	for _, sink := range m.sinks {
		sink.Close()
	}
}
