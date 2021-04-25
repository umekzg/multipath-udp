package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/muxfd/multipath-udp/pkg/demuxer"
)

var (
	input  string
	output string
)

func showHelp() {
	fmt.Printf("Usage:%s {params}\n", os.Args[0])
	fmt.Println("      -c {config file}")
	fmt.Println("      -a {listen addr}")
	fmt.Println("      -h (show help info)")
}

func parse() bool {
	flag.StringVar(&input, "i", "0.0.0.0:1935", "address to use")
	flag.StringVar(&output, "o", "127.0.0.1:1985", "address to use")
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

	outputAddr, err := net.ResolveUDPAddr("udp", output)
	if err != nil {
		panic(err)
	}

	fmt.Printf("listening to %s forwarding to %s\n", inputAddr, outputAddr)

	demuxer.NewDemuxer(demuxer.AutoBindInterfaces()).Start(inputAddr, outputAddr)

	select {}
}
