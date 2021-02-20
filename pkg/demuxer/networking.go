package demuxer

import (
	"fmt"
	"net"
	"time"
)

func GetLocalUDPAddresses(source func() ([]net.Interface, error)) ([]*net.UDPAddr, error) {
	var addrs []*net.UDPAddr
	ifaces, err := source()
	if err != nil {
		return nil, err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			switch v := a.(type) {
			case *net.UDPAddr:
				addrs = append(addrs, v)
			}
		}
	}
	return addrs, nil
}

func makeSet(s []*net.UDPAddr) map[*net.UDPAddr]bool {
	m := make(map[*net.UDPAddr]bool)
	for _, a := range s {
		m[a] = true
	}
	return m
}

func diff(a, b map[*net.UDPAddr]bool) []*net.UDPAddr {
	var d []*net.UDPAddr

	for x := range a {
		if !b[x] {
			d = append(d, x)
		}
	}

	return d
}

func AutoBindInterfaces(source func() ([]net.Interface, error), pollPeriod time.Duration) func(*Demuxer) {
	return func(d *Demuxer) {
		currAddrs, err := GetLocalUDPAddresses(source)
		if err != nil {
			fmt.Printf("error fetching local addresses: %v\n", err)
			return
		}
		currAddrSet := makeSet(currAddrs)
		for iface := range currAddrSet {
			d.AddInterface(iface)
		}
		quit := make(chan bool)

		go func() {
			for {
				select {
				case <-quit:
					return
				case <-time.After(pollPeriod):
					nextAddrs, err := GetLocalUDPAddresses(source)
					if err != nil {
						fmt.Printf("error fetching local addresses: %v\n", err)
					}
					nextAddrSet := makeSet(nextAddrs)
					for _, addr := range diff(currAddrSet, nextAddrSet) {
						d.RemoveInterface(addr)
					}
					for _, addr := range diff(nextAddrSet, currAddrSet) {
						d.AddInterface(addr)
					}
					currAddrs = nextAddrs
					currAddrSet = nextAddrSet
				}
			}
		}()

		d.Wait()
		quit <- true
	}
}
