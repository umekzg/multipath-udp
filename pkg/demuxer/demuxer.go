package demuxer

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/muxfd/multipath-udp/pkg/librtmp"
	"github.com/muxfd/multipath-udp/pkg/protocol"
)

// Demuxer represents a UDP stream demuxer that demuxes a source over multiple senders.
type Demuxer struct {
	interfaces     *InterfaceSet
	rtmp           *librtmp.RTMP
	done           *sync.WaitGroup
	sequenceNumber uint64
}

// NewDemuxer creates a new demuxer.
func NewDemuxer(url string, dial *net.UDPAddr, options ...func(*Demuxer)) *Demuxer {
	r, err := librtmp.Alloc()
	if err != nil {
		panic(err)
	}

	r.Init()

	if err = r.SetupURL(url); err != nil {
		panic(err)
	}

	if err = r.Connect(); err != nil {
		panic(err)
	}

	if !r.IsConnected() {
		panic("not connected")
	}

	var wg sync.WaitGroup
	d := &Demuxer{
		interfaces:     NewInterfaceSet(url[(strings.LastIndex(url, "/")+1):], dial),
		rtmp:           r,
		sequenceNumber: uint64(rand.Uint32()),
		done:           &wg,
	}

	wg.Add(1)

	for _, option := range options {
		option(d)
	}

	go d.readLoop(dial)

	return d
}

func (d *Demuxer) readLoop(dial *net.UDPAddr) {
	defer d.done.Done()
	b := make([]byte, 64*1024)

	for {
		n, err := d.rtmp.Read(b)
		if err != nil || n == 0 {
			break
		}
		d.Write(b[:n])
	}

	if err := d.rtmp.Close(); err != nil {
		fmt.Printf("error closing %v\n", err)
	}

	if err := d.rtmp.Free(); err != nil {
		fmt.Printf("error freeing %v\n", err)
	}
}

func (d *Demuxer) Write(b []byte) {
	packet := protocol.DataPacket{SequenceNumber: d.sequenceNumber, Timestamp: time.Now(), Payload: b}
	d.sequenceNumber += 1
	for _, sender := range d.interfaces.Senders() {
		if _, err := sender.Write(packet); err != nil {
			fmt.Printf("error writing to sender %v: %v\n", sender, err)
		}
	}
}

// Wait waits for the demuxer to exit.
func (d *Demuxer) Wait() {
	d.done.Wait()
}

// Close closes all receivers and sinks associated with the muxer, freeing up resources.
func (d *Demuxer) Close() {

}
