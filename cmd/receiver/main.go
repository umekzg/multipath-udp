package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"

	"github.com/muxfd/multipath-udp/pkg/muxer"
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
	flag.StringVar(&input, "i", "0.0.0.0:1985", "address to listen on")
	flag.StringVar(&output, "o", "127.0.0.1:1935", "address to write to")
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

	runtime.GOMAXPROCS(runtime.NumCPU())

	inputAddr, err := net.ResolveUDPAddr("udp", input)
	if err != nil {
		panic(err)
	}

	outputAddr, err := net.ResolveUDPAddr("udp", output)
	if err != nil {
		panic(err)
	}

	fmt.Printf("listening to %s forwarding to %s\n", inputAddr, outputAddr)

	muxer.NewMuxer().Start(inputAddr, outputAddr)

	select {}
}
