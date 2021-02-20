package main

import (
	"fmt"
	"net"
)

var (
	input  string
	output string
)

func main() {
	inputAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:2345")
	if err != nil {
		panic(err)
	}

	inputConn, err := net.ListenUDP("udp", inputAddr)
	if err != nil {
		panic(err)
	}

	defer inputConn.Close()

	for {
		msg := make([]byte, 2048)

		_, sender, _ := inputConn.ReadFromUDP(msg)

		fmt.Printf("received message from udp address %s: %v\n", sender, msg)
	}
}
