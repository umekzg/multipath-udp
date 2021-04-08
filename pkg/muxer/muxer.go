package muxer

import (
	"fmt"
	"net"
	"sync"

	"github.com/muxfd/multipath-udp/pkg/protocol"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

type Muxer struct {
	sync.RWMutex

	sinks    map[string]*Sink
	sessions map[uint32]*SessionSenderSet

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
		conn: conn,
		done: &wg,
	}
	wg.Add(1)

	for _, option := range options {
		option(m)
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

		h := &rtp.Header{}

		if err := h.Unmarshal(msg[:n]); err != nil {
			fmt.Printf("error unmarshalling rtp packet %v\n", err)
			continue
		}

		if h.PayloadType == 200 {
			// this is an RTCP packet.
			p, err := rtcp.Unmarshal((msg[:n])
			if err != nil {
				fmt.Printf("error unmarshalling rtcp packet %v\n", err)
				continue
			}
		}

		m.sessions[h.SSRC].Add(senderAddr)

		packetType, packet := protocol.Deserialize(msg[:n])

		switch packetType {
		case protocol.HANDSHAKE:
			m.RLock()
			m.conn.WriteToUDP(msg[:n], senderAddr)

			sessionID := packet.(protocol.HandshakePacket).SessionID
			m.sessions[senderAddr.String()] = sessionID
			if _, ok := m.sinks[sessionID]; !ok {
				m.sinks[sessionID] = NewSink(dial)
			}

			m.RUnlock()
			break
		case protocol.DATA:
			// get the sink for this sender.
			sessionID, ok := m.sessions[senderAddr.String()]
			if !ok {
				fmt.Printf("illegal state, no session id received\n")
				break
			}
			sink, ok := m.sinks[sessionID]
			if !ok {
				fmt.Printf("illegal state, no sink for session id: %s\n", sessionID)
				break
			}
			if _, err := sink.Write(packet.(protocol.DataPacket)); err != nil {
				fmt.Printf("error writing packet to sink: %v\n", err)
			}
			break
		default:
			fmt.Printf("unknown packet type %v\n", packetType)
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
