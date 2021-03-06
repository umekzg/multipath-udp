package demuxer

import (
	"net"
	"time"

	"github.com/muxfd/multipath-udp/pkg/interfaces"
)

// AutoBindInterfaces adds a network interface listener to automatically
// add or remove network interfaces.
func AutoBindInterfaces() func(*Demuxer) {
	return func(d *Demuxer) {
		d.interfaceBinder = interfaces.NewAutoBinder(
			interfaces.USBInterfaces(net.Interfaces), 3*time.Second)
	}
}
