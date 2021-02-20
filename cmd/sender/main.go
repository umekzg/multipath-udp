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
	flag.StringVar(&input, "i", "127.0.0.1:1934", "address to use")
	flag.StringVar(&output, "o", "127.0.0.1:1936", "address to use")
	help := flag.Bool("h", false, "help info")
	flag.Parse()

	if *help {
		showHelp()
		return false
	}
	return true
}

func localAddresses() ([]*net.UDPAddr, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var broadcastAddrs []*net.UDPAddr
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			switch v := a.(type) {
			case *net.IPNet:
				broadcastAddrs = append(broadcastAddrs, &net.UDPAddr{IP: v.IP})
			}
		}
	}
	return broadcastAddrs, nil
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

	m := demuxer.NewDemuxer(inputAddr, outputAddr)

	m.Wait()
}
