package muxer

import (
	"bytes"
	"net"
	"testing"

	"github.com/muxfd/multipath-udp/pkg/protocol"
	"go.uber.org/goleak"
)

var (
	muxer      *Muxer
	input1Conn *net.UDPConn
	input2Conn *net.UDPConn
	input3Conn *net.UDPConn
	outputConn *net.UDPConn
)

func setUp(t *testing.T) {
	input := "127.0.0.1:13000"
	output := "127.0.0.1:13001"

	inputAddr, err := net.ResolveUDPAddr("udp", input)
	if err != nil {
		t.Errorf("failed to resolve %s: %v", input, err)
		return
	}

	outputAddr, err := net.ResolveUDPAddr("udp", output)
	if err != nil {
		t.Errorf("failed to resolve %s: %v", output, err)
		return
	}

	muxer = NewMuxer(inputAddr, outputAddr)

	input1Conn, err = net.DialUDP("udp", nil, inputAddr)
	if err != nil {
		t.Errorf("failed to dial %s: %v", inputAddr, err)
		return
	}

	input2Conn, err = net.DialUDP("udp", nil, inputAddr)
	if err != nil {
		t.Errorf("failed to dial %s: %v", inputAddr, err)
		return
	}

	input3Conn, err = net.DialUDP("udp", nil, inputAddr)
	if err != nil {
		t.Errorf("failed to dial %s: %v", inputAddr, err)
		return
	}

	outputConn, err = net.ListenUDP("udp", outputAddr)
	if err != nil {
		t.Errorf("failed to listen %s: %v", outputAddr, err)
		return
	}
}

func tearDown(t *testing.T) {
	muxer.Close()
	outputConn.Close()
	input1Conn.Close()
	input2Conn.Close()
	input3Conn.Close()
	goleak.VerifyNone(t)
}

func expectRead(t *testing.T, conn *net.UDPConn, want []byte) *net.UDPAddr {
	msg := make([]byte, 2048)
	n, sender, err := conn.ReadFromUDP(msg)
	if err != nil {
		t.Errorf("received error from udp address %s: %v\n", sender, err)
	} else if !bytes.Equal(msg[:n], want) {
		t.Errorf("received incorrect read: %s vs %s", msg[:n], want)
	}
	return sender
}

func TestMuxerSingleReceiver_SingleReceiver(t *testing.T) {
	setUp(t)

	handshake, _ := protocol.NewHandshake("moogit sink", []byte{}).Serialize()
	input1Conn.Write(handshake)
	input1Conn.Write([]byte("mooogit test"))

	expectRead(t, outputConn, []byte("mooogit test"))

	tearDown(t)
}

func TestMuxer_SingleReceiver_DuplicateHandshake(t *testing.T) {
	setUp(t)

	handshake, _ := protocol.NewHandshake("moogit sink", []byte{}).Serialize()
	input1Conn.Write(handshake)
	input1Conn.Write(handshake)
	input1Conn.Write(handshake)
	input1Conn.Write([]byte("mooogit test"))

	expectRead(t, outputConn, []byte("mooogit test"))

	tearDown(t)
}

func TestMuxer_SingleReceiver_RespondsOnReceiverWithHandshakes(t *testing.T) {
	setUp(t)

	handshake, _ := protocol.NewHandshake("moogit sink", []byte{}).Serialize()
	for i := 0; i < 10; i++ {
		input1Conn.Write(handshake)

		expectRead(t, input1Conn, handshake)
	}

	tearDown(t)
}

func TestMuxer_SingleReceiver_ForwardsResponse(t *testing.T) {
	setUp(t)

	handshake, _ := protocol.NewHandshake("moogit sink", []byte{}).Serialize()
	input1Conn.Write(handshake)

	expectRead(t, input1Conn, handshake)

	input1Conn.Write([]byte("mooogit test"))

	sender := expectRead(t, outputConn, []byte("mooogit test"))

	outputConn.WriteToUDP([]byte("mooogit response"), sender)

	expectRead(t, input1Conn, []byte("mooogit response"))

	tearDown(t)
}

func TestMuxer_SingleReceiver_E2E(t *testing.T) {
	setUp(t)

	handshake, _ := protocol.NewHandshake("moogit sink", []byte{}).Serialize()
	input1Conn.Write(handshake)
	input1Conn.Write(handshake)
	input1Conn.Write(handshake)

	expectRead(t, input1Conn, handshake)
	expectRead(t, input1Conn, handshake)
	expectRead(t, input1Conn, handshake)

	input1Conn.Write([]byte("mooogit test 1"))
	input1Conn.Write([]byte("mooogit test 1"))
	input1Conn.Write([]byte("mooogit test 1"))

	sender := expectRead(t, outputConn, []byte("mooogit test 1"))
	expectRead(t, outputConn, []byte("mooogit test 1"))
	expectRead(t, outputConn, []byte("mooogit test 1"))

	outputConn.WriteToUDP([]byte("mooogit response"), sender)
	outputConn.WriteToUDP([]byte("mooogit response"), sender)
	outputConn.WriteToUDP([]byte("mooogit response"), sender)

	expectRead(t, input1Conn, []byte("mooogit response"))
	expectRead(t, input1Conn, []byte("mooogit response"))
	expectRead(t, input1Conn, []byte("mooogit response"))

	input1Conn.Write([]byte("mooogit test 2"))
	input1Conn.Write([]byte("mooogit test 2"))
	input1Conn.Write([]byte("mooogit test 3"))

	expectRead(t, outputConn, []byte("mooogit test 2"))
	expectRead(t, outputConn, []byte("mooogit test 2"))
	expectRead(t, outputConn, []byte("mooogit test 3"))

	outputConn.WriteToUDP([]byte("mooogit response"), sender)
	outputConn.WriteToUDP([]byte("mooogit response"), sender)
	outputConn.WriteToUDP([]byte("mooogit response"), sender)

	expectRead(t, input1Conn, []byte("mooogit response"))
	expectRead(t, input1Conn, []byte("mooogit response"))
	expectRead(t, input1Conn, []byte("mooogit response"))

	tearDown(t)
}

func TestMuxer_MultiReceiver_E2E(t *testing.T) {
	setUp(t)

	handshake1, _ := protocol.NewHandshake("moogit sink 1", []byte{1}).Serialize()
	input1Conn.Write(handshake1)
	input1Conn.Write(handshake1)
	input1Conn.Write(handshake1)

	expectRead(t, input1Conn, handshake1)
	expectRead(t, input1Conn, handshake1)
	expectRead(t, input1Conn, handshake1)

	handshake2, _ := protocol.NewHandshake("moogit sink 2", []byte{1}).Serialize()
	input2Conn.Write(handshake2)
	input2Conn.Write(handshake2)
	input2Conn.Write(handshake2)

	expectRead(t, input2Conn, handshake2)
	expectRead(t, input2Conn, handshake2)
	expectRead(t, input2Conn, handshake2)

	input1Conn.Write([]byte("mooogit test 1"))
	input1Conn.Write([]byte("mooogit test 1"))
	input2Conn.Write([]byte("mooogit test 1"))
	input2Conn.Write([]byte("mooogit test 1"))

	sender := expectRead(t, outputConn, []byte("mooogit test 1"))
	expectRead(t, outputConn, []byte("mooogit test 1"))
	expectRead(t, outputConn, []byte("mooogit test 1"))
	expectRead(t, outputConn, []byte("mooogit test 1"))

	outputConn.WriteToUDP([]byte("mooogit response"), sender)
	outputConn.WriteToUDP([]byte("mooogit response"), sender)

	expectRead(t, input1Conn, []byte("mooogit response"))
	expectRead(t, input1Conn, []byte("mooogit response"))

	expectRead(t, input2Conn, []byte("mooogit response"))
	expectRead(t, input2Conn, []byte("mooogit response"))

	input1Conn.Write([]byte("mooogit test 2"))
	input1Conn.Write([]byte("mooogit test 2"))
	input2Conn.Write([]byte("mooogit test 2"))
	input2Conn.Write([]byte("mooogit test 3"))
	input1Conn.Write([]byte("mooogit test 3"))

	expectRead(t, outputConn, []byte("mooogit test 2"))
	expectRead(t, outputConn, []byte("mooogit test 2"))
	expectRead(t, outputConn, []byte("mooogit test 2"))
	expectRead(t, outputConn, []byte("mooogit test 3"))
	expectRead(t, outputConn, []byte("mooogit test 3"))

	handshake3, _ := protocol.NewHandshake("moogit sink 3", []byte{1}).Serialize()
	input3Conn.Write(handshake3)
	input3Conn.Write(handshake3)
	input3Conn.Write(handshake3)

	expectRead(t, input3Conn, handshake3)
	expectRead(t, input3Conn, handshake3)
	expectRead(t, input3Conn, handshake3)

	outputConn.WriteToUDP([]byte("mooogit response 2"), sender)
	outputConn.WriteToUDP([]byte("mooogit response 2"), sender)
	outputConn.WriteToUDP([]byte("mooogit response 2"), sender)

	expectRead(t, input1Conn, []byte("mooogit response 2"))
	expectRead(t, input1Conn, []byte("mooogit response 2"))
	expectRead(t, input1Conn, []byte("mooogit response 2"))

	expectRead(t, input2Conn, []byte("mooogit response 2"))
	expectRead(t, input2Conn, []byte("mooogit response 2"))
	expectRead(t, input2Conn, []byte("mooogit response 2"))

	expectRead(t, input3Conn, []byte("mooogit response 2"))
	expectRead(t, input3Conn, []byte("mooogit response 2"))
	expectRead(t, input3Conn, []byte("mooogit response 2"))

	tearDown(t)
}

func TestMuxer_MultiReceiver_MultiSession_E2E(t *testing.T) {
	setUp(t)

	handshake1, _ := protocol.NewHandshake("moogit sink 1", []byte{1}).Serialize()
	input1Conn.Write(handshake1)
	expectRead(t, input1Conn, handshake1)

	handshake2, _ := protocol.NewHandshake("moogit sink 2", []byte{1}).Serialize()
	input2Conn.Write(handshake2)
	expectRead(t, input2Conn, handshake2)

	input1Conn.Write([]byte("mooogit test 1"))
	input2Conn.Write([]byte("mooogit test 1"))

	sender1 := expectRead(t, outputConn, []byte("mooogit test 1"))
	sender2 := expectRead(t, outputConn, []byte("mooogit test 1"))

	if sender1.Port != sender2.Port {
		t.Errorf("expected same senders, received %v and %v", sender1, sender2)
	}

	outputConn.WriteToUDP([]byte("mooogit response"), sender1)

	expectRead(t, input1Conn, []byte("mooogit response"))
	expectRead(t, input2Conn, []byte("mooogit response"))

	handshake3, _ := protocol.NewHandshake("moogit sink 3", []byte{2}).Serialize()
	input3Conn.Write(handshake3)

	input3Conn.Write([]byte("mooogit test 1"))

	sender3 := expectRead(t, outputConn, []byte("mooogit test 1"))

	if sender1.Port == sender3.Port {
		t.Errorf("expected different senders, received %v and %v", sender1, sender3)
	}

	tearDown(t)
}
