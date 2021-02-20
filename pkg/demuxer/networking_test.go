package demuxer

import (
	"bytes"
	"net"
	"testing"
	"time"

	"go.uber.org/goleak"
)

var (
	demuxer    *Demuxer
	inputConn  *net.UDPConn
	outputConn *net.UDPConn
)

func setUp(t *testing.T, options ...func(*Demuxer)) {
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

	demuxer = NewDemuxer(inputAddr, outputAddr, options...)

	inputConn, err = net.DialUDP("udp", nil, inputAddr)
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
	demuxer.Close()
	outputConn.Close()
	inputConn.Close()
	goleak.VerifyNone(t)
}

func expectHandshake(t *testing.T, conn *net.UDPConn) *net.UDPAddr {
	msg := make([]byte, 4096)
	n, sender, err := conn.ReadFromUDP(msg)
	if err != nil {
		t.Errorf("received error from udp address %s: %v\n", sender, err)
	}
	_, err = conn.WriteToUDP(msg[:n], sender)
	if err != nil {
		t.Errorf("received error returning handshake from udp address %s: %v\n", sender, err)
	}
	return sender
}

func expectRead(t *testing.T, conn *net.UDPConn, want []byte) *net.UDPAddr {
	msg := make([]byte, 4096)
	n, sender, err := conn.ReadFromUDP(msg)
	if err != nil {
		t.Errorf("received error from udp address %s: %v\n", sender, err)
	} else if !bytes.Equal(msg[:n], want) {
		t.Errorf("received incorrect response: %v vs %v", msg[:n], want)
	}
	return sender
}

func TestDemuxer_SingleSender(t *testing.T) {
	setUp(t)

	demuxer.AddInterface(nil)

	inputConn.Write([]byte("mooogit test"))

	expectHandshake(t, outputConn)

	expectRead(t, outputConn, []byte("mooogit test"))

	tearDown(t)
}

func TestDemuxer_SingleSender_DelayedResponseDuplicateHandshake(t *testing.T) {
	setUp(t, Handshake(1*time.Millisecond)) // 1ms timeout seems reasonable for the networking stack.

	demuxer.AddInterface(nil)

	inputConn.Write([]byte("mooogit test"))

	msg1 := make([]byte, 4096)
	msg2 := make([]byte, 4096)
	msg3 := make([]byte, 4096)
	n, sender, _ := outputConn.ReadFromUDP(msg1)
	outputConn.ReadFromUDP(msg2)
	if !bytes.Equal(msg1, msg2) {
		t.Errorf("invalid second handshake message: %v != %v", msg1, msg2)
	}
	outputConn.ReadFromUDP(msg3)
	if !bytes.Equal(msg1, msg3) {
		t.Errorf("invalid third handshake message: %v != %v", msg1, msg3)
	}

	outputConn.WriteToUDP(msg1[:n], sender)
	outputConn.WriteToUDP(msg1[:n], sender)
	outputConn.WriteToUDP(msg1[:n], sender)

	expectRead(t, outputConn, []byte("mooogit test"))

	tearDown(t)
}

func TestDemuxer_SingleSender_MultipleMessages(t *testing.T) {
	setUp(t)

	demuxer.AddInterface(nil)

	inputConn.Write([]byte("mooogit test 1"))
	inputConn.Write([]byte("mooogit test 2"))
	inputConn.Write([]byte("mooogit test 3"))

	expectHandshake(t, outputConn)

	expectRead(t, outputConn, []byte("mooogit test 1"))
	expectRead(t, outputConn, []byte("mooogit test 2"))
	expectRead(t, outputConn, []byte("mooogit test 3"))

	tearDown(t)
}

func TestDemuxer_SingleSender_ForwardsResponse(t *testing.T) {
	setUp(t)

	demuxer.AddInterface(nil)

	inputConn.Write([]byte("mooogit test 1"))

	expectHandshake(t, outputConn)

	sender := expectRead(t, outputConn, []byte("mooogit test 1"))

	outputConn.WriteToUDP([]byte("mooogit response"), sender)

	expectRead(t, inputConn, []byte("mooogit response"))

	tearDown(t)
}
