package muxer

import (
	"bytes"
	"net"
	"testing"

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

	input1Conn.Write([]byte("mugit sink"))
	input1Conn.Write([]byte("mooogit test"))

	expectRead(t, outputConn, []byte("mooogit test"))

	tearDown(t)
}

func TestMuxer_SingleReceiver_DuplicateHandshake(t *testing.T) {
	setUp(t)

	input1Conn.Write([]byte("mugit sink"))
	input1Conn.Write([]byte("mugit sink"))
	input1Conn.Write([]byte("mugit sink"))
	input1Conn.Write([]byte("mooogit test"))

	expectRead(t, outputConn, []byte("mooogit test"))

	tearDown(t)
}

func TestMuxer_SingleReceiver_RespondsOnReceiverWithHandshakes(t *testing.T) {
	setUp(t)

	for i := 0; i < 10; i++ {
		input1Conn.Write([]byte("mugit sink"))

		expectRead(t, input1Conn, []byte("mugit sink"))
	}

	tearDown(t)
}

func TestMuxer_SingleReceiver_ForwardsResponse(t *testing.T) {
	setUp(t)

	input1Conn.Write([]byte("mugit sink"))

	expectRead(t, input1Conn, []byte("mugit sink"))

	input1Conn.Write([]byte("mooogit test"))

	sender := expectRead(t, outputConn, []byte("mooogit test"))

	outputConn.WriteToUDP([]byte("mooogit response"), sender)

	expectRead(t, input1Conn, []byte("mooogit response"))

	tearDown(t)
}

func TestMuxer_SingleReceiver_E2E(t *testing.T) {
	setUp(t)

	input1Conn.Write([]byte("mugit sink"))
	input1Conn.Write([]byte("mugit sink"))
	input1Conn.Write([]byte("mugit sink"))

	expectRead(t, input1Conn, []byte("mugit sink"))
	expectRead(t, input1Conn, []byte("mugit sink"))
	expectRead(t, input1Conn, []byte("mugit sink"))

	input1Conn.Write([]byte("mooogit test 1"))
	input1Conn.Write([]byte("mooogit test 1"))
	input1Conn.Write([]byte("mooogit test 1"))

	sender := expectRead(t, outputConn, []byte("mooogit test 1"))

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

	input1Conn.Write([]byte("mugit sink"))
	input1Conn.Write([]byte("mugit sink"))
	input1Conn.Write([]byte("mugit sink"))

	expectRead(t, input1Conn, []byte("mugit sink"))
	expectRead(t, input1Conn, []byte("mugit sink"))
	expectRead(t, input1Conn, []byte("mugit sink"))

	input2Conn.Write([]byte("mugit sink"))
	input2Conn.Write([]byte("mugit sink"))
	input2Conn.Write([]byte("mugit sink"))

	expectRead(t, input2Conn, []byte("mugit sink"))
	expectRead(t, input2Conn, []byte("mugit sink"))
	expectRead(t, input2Conn, []byte("mugit sink"))

	input1Conn.Write([]byte("mooogit test 1"))
	input1Conn.Write([]byte("mooogit test 1"))
	input2Conn.Write([]byte("mooogit test 1"))
	input2Conn.Write([]byte("mooogit test 1"))

	sender := expectRead(t, outputConn, []byte("mooogit test 1"))

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
	expectRead(t, outputConn, []byte("mooogit test 3"))

	input3Conn.Write([]byte("mugit sink"))
	input3Conn.Write([]byte("mugit sink"))
	input3Conn.Write([]byte("mugit sink"))

	expectRead(t, input3Conn, []byte("mugit sink"))
	expectRead(t, input3Conn, []byte("mugit sink"))
	expectRead(t, input3Conn, []byte("mugit sink"))

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
