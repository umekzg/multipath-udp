package demuxer

import (
	"bytes"
	"fmt"
	"net"
	"time"
)

// Source represents an inbound UDP socket bound to a session.
type Source struct {
	ID      []byte
	address *net.UDPAddr
	senders *SenderMap

	handshakeTimeout time.Duration

	send chan []byte // unprocessed messages from the sender.
}

// Sender represents a multiplexed UDP session from a single source.
type Sender struct {
	send chan []byte

	conn *net.UDPConn
}

// ReceiveMessage notifies the source of a message.
func (source *Source) ReceiveMessage(b []byte) {
	source.senders.SendAll(b)
}

// AddSender adds a route to raddr via laddr.
func (source *Source) AddSender(laddr *net.UDPAddr, raddr *net.UDPAddr) {
	send := make(chan []byte, 2048)
	conn, err := net.DialUDP("udp", laddr, raddr)
	if err != nil {
		fmt.Printf("error dialing %s -> %s\n", laddr, raddr)
		panic(err)
	}
	sender := &Sender{
		send: send,
		conn: conn,
	}
	source.senders.Set(laddr, sender)

	// TODO: this negotiation can probably be cleaned up a bit...

	// start negotiation
	successfulHandshake := make(chan bool)
	go func() {
		// write the initial handshake immediately.
		_, err := conn.Write(source.ID)
		if err != nil {
			fmt.Printf("failed to write handshake address %s: %v\n", raddr, err)
		}
		for {
			select {
			case <-successfulHandshake:
				// pipe write channel
				go func() {
					for {
						msg, ok := <-send
						if !ok {
							fmt.Printf("write loop closed for sender %v %v\n", laddr, raddr)
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
			case <-time.After(source.handshakeTimeout):
				_, err := conn.Write(source.ID)
				if err != nil {
					fmt.Printf("failed to write handshake address %s: %v\n", raddr, err)
				}
			}
		}
	}()

	// read inbound channel
	go func() {
		handshakeReceived := false
		for {
			msg := make([]byte, 2048)
			n, muxer, err := conn.ReadFromUDP(msg)
			if err != nil {
				fmt.Printf("read error from udp address %s: %v\n", muxer, err)
				break
			}
			if !handshakeReceived {
				if !bytes.Equal(msg[:n], source.ID) {
					fmt.Printf("invalid handshake from udp address %s: %v\n", raddr, msg[:n])
				} else {
					handshakeReceived = true
					successfulHandshake <- true
				}
			} else {
				source.send <- msg[:n]
			}
		}
	}()
}

// Close closes the sender.
func (sender *Sender) Close() {
	sender.conn.Close()
	close(sender.send)
}

// Close closes the source.
func (source *Source) Close() {
	source.senders.CloseAll()
	close(source.send)
}
