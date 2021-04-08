package demuxer

import (
	"net"
	"time"

	"github.com/muxfd/multipath-udp/pkg/interfaces"
)

// HandshakeTimeout specifies the duration before resending a UDP handshake.
func HandshakeTimeout(t time.Duration) func(*Demuxer) {
	return func(d *Demuxer) {
	}
}

// AutoBindInterfaces adds a network interface listener to automatically
// add or remove network interfaces.
func AutoBindInterfaces(raddr string) func(*Demuxer) {
	binder := interfaces.NewAutoBinder(net.Interfaces, 3*time.Second)
	return func(d *Demuxer) {
		close := binder.Bind(d.interfaces.Add, d.interfaces.Remove, raddr)
		go func() {
			d.Wait()
			close()
		}()
	}
}
