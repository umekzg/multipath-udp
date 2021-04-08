package demuxer

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/muxfd/multipath-udp/pkg/protocol"
)

func TestSender_SingleSender(t *testing.T) {
	msg := make([]byte, 4096)

	raddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:13002")
	if err != nil {
		t.Errorf("failed to resolve %s: %v", "127.0.0.1:13002", err)
	}
	conn, err := net.ListenUDP("udp", raddr)
	if err != nil {
		t.Errorf("failed to listen %s: %v", raddr, err)
	}

	sender := NewSender("moo session id", nil, raddr, 1*time.Second)

	n, src, err := conn.ReadFromUDP(msg)
	if err != nil {
		t.Errorf("received error from udp address %v: %v", sender, err)
	}
	// expect a handshake with moo session id.
	packetType, packet := protocol.Deserialize(msg[:n])
	if packetType != protocol.HANDSHAKE {
		t.Errorf("expected handshake but received %v", packetType)
	}
	if packet.(protocol.HandshakePacket).SessionID != "moo session id" {
		t.Errorf("expected handshake %v but received %v", "moo session id", []byte(packet.(protocol.HandshakePacket).SessionID))
	}

	if _, err := conn.WriteToUDP(msg[:n], src); err != nil {
		t.Errorf("failed to write %v", err)
	}

	data := protocol.DataPacket{SequenceNumber: 123, Timestamp: time.Unix(0, time.Now().UnixNano()), Payload: []byte("moo msg")}

	if _, err := sender.Write(data); err != nil {
		t.Errorf("failed to send %v", err)
	}

	n, src, err = conn.ReadFromUDP(msg)
	if err != nil {
		t.Errorf("received error from udp address %v: %v", sender, err)
	}
	// expect a data packet.
	packetType, packet = protocol.Deserialize(msg[:n])
	if packetType != protocol.DATA {
		t.Errorf("expected data but received %v", packetType)
	}
	if !reflect.DeepEqual(packet.(protocol.DataPacket), data) {
		t.Errorf("expected data %v but received %v", data, packet.(protocol.DataPacket))
	}
}
