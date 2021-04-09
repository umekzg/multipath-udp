package buffer

import (
	"testing"
	"time"

	"github.com/muxfd/multipath-udp/pkg/srt"
)

func isBufferEmpty(buf *ReceiverBuffer) bool {
	return buf.buffer[buf.tail] == nil
}

func TestReceiverBuffer_Simple(t *testing.T) {
	buf := NewReceiverBuffer(100 * time.Millisecond)
	p1 := &srt.Packet{
		SequenceNumber: 100,
	}

	// check that the buffer emits all three packets after two seconds.
	if !isBufferEmpty(buf) {
		t.Errorf("expected empty buffer")
	}

	buf.Add(p1)

	select {
	case <-buf.EmitCh:
		t.Errorf("emission received too early")
		return
	case <-time.After(95 * time.Millisecond):
		break
	}

	ts1 := time.Now()

	val1 := <-buf.EmitCh
	if val1 != p1 {
		t.Errorf("expected %v, got %v", p1, val1)
	}

	ts2 := time.Now()

	if !isBufferEmpty(buf) {
		t.Errorf("expected empty buffer")
	}

	if len(buf.EmitCh) > 0 {
		t.Errorf("expected empty emit channel")
	}

	if ts2.Sub(ts1) > 10*time.Millisecond {
		t.Errorf("took too long to emit")
	}

	if len(buf.LossCh) > 0 {
		t.Errorf("expected no retransmissions")
	}

	buf.Close()
	// goleak.VerifyNone(t)
}

func TestReceiverBuffer_TwoPackets(t *testing.T) {
	buf := NewReceiverBuffer(100 * time.Millisecond)
	p1 := &srt.Packet{
		SequenceNumber: 100,
	}
	p2 := &srt.Packet{
		SequenceNumber: 101,
	}

	// check that the buffer emits all three packets after two seconds.
	if !isBufferEmpty(buf) {
		t.Errorf("expected empty buffer")
	}

	buf.Add(p1)
	buf.Add(p2)

	select {
	case <-buf.EmitCh:
		t.Errorf("emission received too early")
		return
	case <-time.After(95 * time.Millisecond):
		break
	}

	ts1 := time.Now()

	val1 := <-buf.EmitCh
	if val1 != p1 {
		t.Errorf("expected %v, got %v", p1, val1)
	}
	val2 := <-buf.EmitCh
	if val2 != p2 {
		t.Errorf("expected %v, got %v", p2, val2)
	}

	ts2 := time.Now()

	if !isBufferEmpty(buf) {
		t.Errorf("expected empty buffer")
	}

	if len(buf.EmitCh) > 0 {
		t.Errorf("expected empty emit channel")
	}

	if ts2.Sub(ts1) > 10*time.Millisecond {
		t.Errorf("took too long to emit")
	}

	if len(buf.LossCh) > 0 {
		t.Errorf("expected no retransmissions")
	}

	buf.Close()
	// goleak.VerifyNone(t)
}

func TestReceiverBuffer_Deduplicates(t *testing.T) {
	buf := NewReceiverBuffer(100 * time.Millisecond)
	p1 := &srt.Packet{
		SequenceNumber: 100,
	}
	p2 := &srt.Packet{
		SequenceNumber: 101,
	}
	p3 := &srt.Packet{
		SequenceNumber: 100,
	}

	// check that the buffer emits all three packets after two seconds.
	if !isBufferEmpty(buf) {
		t.Errorf("expected empty buffer")
	}

	buf.Add(p1)
	buf.Add(p2)
	buf.Add(p3)

	select {
	case <-buf.EmitCh:
		t.Errorf("emission received too early")
		return
	case <-time.After(95 * time.Millisecond):
		break
	}

	ts1 := time.Now()

	val1 := <-buf.EmitCh
	if val1 != p1 {
		t.Errorf("expected %v, got %v", p1, val1)
	}
	val2 := <-buf.EmitCh
	if val2 != p2 {
		t.Errorf("expected %v, got %v", p2, val2)
	}

	ts2 := time.Now()

	if !isBufferEmpty(buf) {
		t.Errorf("expected empty buffer")
	}

	if len(buf.EmitCh) > 0 {
		t.Errorf("expected empty emit channel")
	}

	if ts2.Sub(ts1) > 10*time.Millisecond {
		t.Errorf("took too long to emit")
	}

	if len(buf.LossCh) > 0 {
		t.Errorf("expected no retransmissions")
	}

	buf.Close()
	// goleak.VerifyNone(t)
}
