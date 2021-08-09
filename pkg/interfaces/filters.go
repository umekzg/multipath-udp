package interfaces

import (
	"net"
	"strings"
)

func USBInterfaces(source func() ([]net.Interface, error)) func() ([]net.Interface, error) {
	return func() ([]net.Interface, error) {
		ifaces, err := source()
		if err != nil {
			return nil, err
		}
		var filtered []net.Interface
		for _, iface := range ifaces {
			if strings.HasPrefix(iface.Name, "usb") || (strings.HasPrefix(iface.Name, "wlan") && iface.Name != "wlan0") {
				filtered = append(filtered, iface)
			}
		}
		return filtered, nil
	}
}
