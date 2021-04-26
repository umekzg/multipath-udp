package meter

import (
	"fmt"
	"net"
	"time"
)

type Meter struct {
	interval   time.Duration
	expiration time.Time
	counts     map[string]uint32
	senders    map[string]*net.UDPAddr
}

func NewMeter(interval time.Duration) *Meter {
	expiration := time.Now().Add(interval)
	counts := make(map[string]uint32)
	senders := make(map[string]*net.UDPAddr)

	return &Meter{
		interval:   interval,
		expiration: expiration,
		counts:     counts,
		senders:    senders,
	}
}

func (m *Meter) IsExpired() bool {
	return m.expiration.Before(time.Now())
}

func (m *Meter) Expire(sink func(*net.UDPAddr, uint32)) {
	for senderAddr, ct := range m.counts {
		sender, ok := m.senders[senderAddr]
		if !ok {
			fmt.Printf("lmao wtf\n")
			continue
		}
		go sink(sender, ct)
	}
	// m.counts = make(map[string]uint32)
	m.expiration = time.Now().Add(m.interval)
}

func (m *Meter) Increment(addr *net.UDPAddr) {
	s := addr.String()
	m.counts[s] += 1
	m.senders[s] = addr
}
