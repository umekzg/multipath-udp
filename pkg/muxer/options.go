package muxer

import (
	"github.com/muxfd/multipath-udp/pkg/deduplicator"
	"github.com/muxfd/multipath-udp/pkg/scheduler"
)

func WithScheduler(scheduler scheduler.Scheduler) func(*Muxer) {
	return func(d *Muxer) {
		d.scheduler = scheduler
	}
}

func WithDeduplicator(deduplicator deduplicator.Deduplicator) func(*Muxer) {
	return func(d *Muxer) {
		d.deduplicator = deduplicator
	}
}
