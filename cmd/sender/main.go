package main

import (
	"flag"
	"fmt"
	"net"
	"os"
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
	broadcastAddrs, err := localAddresses()
	if err != nil {
		fmt.Printf("error finding addresses for broadcast: %v\n", err)
		os.Exit(-1)
	}

	if !parse() {
		showHelp()
		os.Exit(-1)
	}

	inputAddr, err := net.ResolveUDPAddr("udp", input)
	if err != nil {
		fmt.Printf("could not resolve address %s: %v\n", input, err)
		os.Exit(-1)
	}

	outputAddr, err := net.ResolveUDPAddr("udp", output)
	if err != nil {
		fmt.Printf("could not resolve output address %s: %v\n", output, err)
		os.Exit(-1)
	}

	conn, err := net.ListenUDP("udp", inputAddr)
	if err != nil {
		fmt.Printf("failed to listen on address %s: %v\n", inputAddr, err)
		os.Exit(-1)
	}

	defer conn.Close()

	var dialers []*net.UDPConn

	for _, broadcastAddr := range broadcastAddrs {
		dialer, err := net.DialUDP("udp", broadcastAddr, outputAddr)
		if err == nil {
			defer dialer.Close()

			dialers = append(dialers, dialer)
		}
		// it's ok if the dialer creation fails.
	}

	msg := make([]byte, 2048)

	for {
		len, _, err := conn.ReadFromUDP(msg)
		if err != nil {
			fmt.Printf("failed to read udp packet: %v\n", err)
			os.Exit(-1)
		}

		for _, dialer := range dialers {
			dialer.Write(msg[:len])
		}
	}
}
