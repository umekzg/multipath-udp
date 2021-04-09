package demuxer

import (
	"fmt"
	"net"
	"time"

	"github.com/muxfd/multipath-udp/pkg/protocol"
)

// Sender represents a multiplexed UDP session from a single source.
type Sender struct {
	MissingSequences chan uint64
	send             chan protocol.DataPacket
	conn             *net.UDPConn
}

// AddSender adds a route to raddr via laddr.
func NewSender(sessionID string, laddr, raddr *net.UDPAddr, handshakeTimeout time.Duration) *Sender {
	send := make(chan protocol.DataPacket, 128)
	conn, err := net.DialUDP("udp", laddr, raddr)
	if err != nil {
		fmt.Printf("error dialing %s -> %s\n", laddr, raddr)
		panic(err)
	}
	conn.SetReadBuffer(1024 * 1024)
	conn.SetWriteBuffer(1024 * 1024)
	sender := &Sender{
		send: send,
		conn: conn,
	}

	// TODO: this negotiation can probably be cleaned up a bit...
	// start negotiation
	successfulHandshake := make(chan bool)
	handshake := protocol.HandshakePacket{SessionID: sessionID}.Serialize()
	go func() {
		// write the initial handshake immediately.
		_, err := conn.Write(handshake)
		if err != nil {
			fmt.Printf("failed to write handshake address %s: %v\n", raddr, err)
		}
		for attempts := 0; attempts < 30; attempts++ {
			select {
			case <-successfulHandshake:
				// pipe write channel
				go func() {
					for {
						msg, ok := <-send
						if !ok {
							break
						}
						_, err := conn.Write(msg.Serialize())
						if err != nil {
							fmt.Printf("failed to write msg %v for sender %v %v\n", msg, laddr, raddr)
						}
					}
				}()
				close(successfulHandshake)
				return
			case <-time.After(handshakeTimeout):
				_, err := conn.Write(handshake)
				if err != nil {
					fmt.Printf("failed to write handshake address %s: %v\n", raddr, err)
				}
			}
		}
		fmt.Printf("failed to establish handshake on address %s\n", raddr)
	}()

	// read inbound channel
	go func() {
		handshakeReceived := false
		msg := make([]byte, 2048)
		for {
			n, _, err := conn.ReadFromUDP(msg)
			if err != nil {
				break
			}
			packetType, _ := protocol.Deserialize(msg[:n])
			switch packetType {
			case protocol.HANDSHAKE:
				if !handshakeReceived {
					// don't push duplicates, just ignore them.
					successfulHandshake <- true
				}
				handshakeReceived = true
			case protocol.MISSING_SEQUENCE:
				// resend the missing sequence.
			case protocol.METER:
				// take note and drop packets if congested.
			default:
				// shouldn't happen lol.
				fmt.Printf("received invalid data packet from %s\n", raddr)
			}
		}
	}()

	return sender
}

func (sender *Sender) Write(msg protocol.DataPacket) (n int, err error) {
	sender.send <- msg
	return len(msg.Payload), nil
}

// Close closes the sender.
func (sender *Sender) Close() {
	sender.conn.Close()
	close(sender.send)
}
