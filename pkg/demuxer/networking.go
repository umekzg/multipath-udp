package demuxer

import (
	"net"
	"time"

	"github.com/muxfd/multipath-udp/pkg/networking"
)

func AutoBindInterfaces() func(*Demuxer) {
	binder := networking.NewAutoBinder(net.Interfaces, 15*time.Second)
	return func(d *Demuxer) {
		close := binder.Bind(d.AddInterface, d.RemoveInterface)
		d.Wait()
		close()
	}
}
