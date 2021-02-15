package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	lru "github.com/hashicorp/golang-lru"
	fnv1a "github.com/segmentio/fasthash/fnv1a"
)

var (
	input  string
	output string
)

func showHelp() {
	fmt.Printf("Usage:%s {params}\n", os.Args[0])
	fmt.Println("      -i {addr}")
	fmt.Println("      -o {addr}")
	fmt.Println("      -h (show help info)")
}

func parse() bool {
	flag.StringVar(&input, "i", "127.0.0.1:1936", "address to listen on")
	flag.StringVar(&output, "o", "127.0.0.1:1937", "address to write to")
	help := flag.Bool("h", false, "help info")
	flag.Parse()

	if *help {
		showHelp()
		return false
	}
	return true
}

func main() {
	if !parse() {
		showHelp()
		os.Exit(-1)
	}

	inputAddr, err := net.ResolveUDPAddr("udp", input)
	if err != nil {
		panic(err)
	}

	inputConn, err := net.ListenUDP("udp", inputAddr)
	if err != nil {
		panic(err)
	}

	outputConn, err := net.Dial("udp", output)
	if err != nil {
		panic(err)
	}

	defer inputConn.Close()
	defer outputConn.Close()

	msg := make([]byte, 2048)

	cache, err := lru.New(100000000)
	if err != nil {
		panic(err)
	}

	for {
		len, _, err := inputConn.ReadFromUDP(msg)
		if err != nil {
			panic(err)
		}

		if ok, _ := cache.ContainsOrAdd(fnv1a.HashBytes64(msg[:len]), true); ok {
			// ignore duplicate packets.
			continue
		}

		outputConn.Write(msg[:len])
	}
}
