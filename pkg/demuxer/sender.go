package demuxer

import (
	"bytes"
	"fmt"
	"net"
	"time"
)

// Sender represents a multiplexed UDP session from a single source.
type Sender struct {
	send   chan []byte
	conn   *net.UDPConn
	closed bool
}

// AddSender adds a route to raddr via laddr.
func NewSender(handshake []byte, laddr, raddr *net.UDPAddr, onResponse func([]byte), handshakeTimeout time.Duration) *Sender {
	send := make(chan []byte, 128)
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
						_, err := conn.Write(msg)
						if err != nil {
							fmt.Printf("failed to write msg %s for sender %v %v\n", msg, laddr, raddr)
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
			isHandshake := bytes.Equal(msg[:n], handshake)
			if !handshakeReceived {
				if !isHandshake {
					fmt.Printf("invalid handshake from udp address %s: %v\n", raddr, msg[:n])
				} else {
					handshakeReceived = true
					successfulHandshake <- true
				}
			} else if !isHandshake {
				onResponse(msg[:n])
			} else {
				fmt.Printf("duplicate handshake received\n")
			}
		}
	}()

	return sender
}

func (sender *Sender) Write(msg []byte) (n int, err error) {
	sender.send <- msg
	return len(msg), nil
}

// Close closes the sender.
func (sender *Sender) Close() {
	sender.conn.Close()
	close(sender.send)
}
