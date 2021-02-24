package networking

import (
	"fmt"
	"net"
	"strings"
	"time"
)

func tryDial(laddr *net.UDPAddr, raddr string) bool {
	d := net.Dialer{LocalAddr: laddr, Timeout: 5 * time.Second}
	_, err := d.Dial("udp", raddr)
	return err == nil
}

func getAddresses(source func() ([]net.Interface, error), raddr string) ([]*net.UDPAddr, error) {
	var addrs []*net.UDPAddr
	ifaces, err := source()
	if err != nil {
		return nil, err
	}
	for _, i := range ifaces {
		if strings.HasPrefix(i.Name, "docker") {
			// this is a virtual connection, ignore.
			continue
		}
		ifaceAddrs, err := i.Addrs()
		if err != nil {
			fmt.Printf("error getting interface addresses: %v\n", err)
			continue
		}
		for _, a := range ifaceAddrs {
			switch v := a.(type) {
			case *net.IPNet:
				addr := &net.UDPAddr{IP: v.IP}
				if tryDial(addr, raddr) {
					addrs = append(addrs, addr)
				}
			case *net.UDPAddr:
				if tryDial(v, raddr) {
					addrs = append(addrs, v)
				}
			}
		}
	}
	return addrs, nil
}

func makeSet(s []*net.UDPAddr) map[string]*net.UDPAddr {
	m := make(map[string]*net.UDPAddr)
	for _, a := range s {
		m[a.String()] = a
	}
	return m
}

func diff(a, b map[string]*net.UDPAddr) []*net.UDPAddr {
	var d []*net.UDPAddr

	for k, x := range a {
		if _, found := b[k]; !found {
			d = append(d, x)
		}
	}

	return d
}

// AutoBinder is a tool to automatically add/remove network interfaces
// as they are added/removed to the host device.
type AutoBinder struct {
	source     func() ([]net.Interface, error)
	pollPeriod time.Duration
}

// NewAutoBinder returns a new AutoBinder with the given config.
func NewAutoBinder(source func() ([]net.Interface, error), pollPeriod time.Duration) *AutoBinder {
	return &AutoBinder{
		source:     source,
		pollPeriod: pollPeriod,
	}
}

// Bind begins binding the AutoBinder to the two difference functions,
// calling add when a new interface is added and sub when an interface is removed.
func (b *AutoBinder) Bind(add, sub func(*net.UDPAddr), raddr string) func() {
	currAddrs, err := getAddresses(b.source, raddr)
	if err != nil {
		fmt.Printf("error fetching local addresses: %v\n", err)
		currAddrs = []*net.UDPAddr{}
	} else if len(currAddrs) == 0 {
		fmt.Printf("no local addresses found\n")
	}
	currAddrSet := makeSet(currAddrs)
	for _, iface := range currAddrSet {
		add(iface)
	}
	quit := make(chan bool)

	go func() {
		for {
			select {
			case <-quit:
				return
			case <-time.After(b.pollPeriod):
				nextAddrs, err := getAddresses(b.source, raddr)
				if err != nil {
					fmt.Printf("error fetching local addresses: %v\n", err)
					break
				}
				nextAddrSet := makeSet(nextAddrs)
				for _, addr := range diff(currAddrSet, nextAddrSet) {
					sub(addr)
				}
				for _, addr := range diff(nextAddrSet, currAddrSet) {
					add(addr)
				}
				currAddrs = nextAddrs
				currAddrSet = nextAddrSet
			}
		}
	}()

	return func() {
		quit <- true
	}
}
