package buffer

import (
	"testing"
)

func TestSenderBuffer(t *testing.T) {
	buf := NewSenderBuffer(5)

	for i := 0; i < 5; i++ {
		if value, err := buf.Get(i); err == nil {
			t.Errorf("expected error, got value %v", value)
		}
	}

	buf.Add(123)

	if value, err := buf.Get(0); value != 123 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	for i := 1; i < 5; i++ {
		if value, err := buf.Get(i); err == nil {
			t.Errorf("expected error, got value %v", value)
		}
	}

	buf.Add(234)

	if value, err := buf.Get(0); value != 123 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	if value, err := buf.Get(1); value != 234 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	for i := 2; i < 5; i++ {
		if value, err := buf.Get(i); err == nil {
			t.Errorf("expected error, got value %v", value)
		}
	}
	buf.Add(345)
	buf.Add(456)
	buf.Add(567)

	if value, err := buf.Get(0); value != 123 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	if value, err := buf.Get(1); value != 234 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	if value, err := buf.Get(2); value != 345 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	if value, err := buf.Get(3); value != 456 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	if value, err := buf.Get(4); value != 567 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}

	buf.Add(678)

	if value, err := buf.Get(0); err == nil {
		t.Errorf("expected error, got value %v", value)
	}
	if value, err := buf.Get(1); value != 234 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	if value, err := buf.Get(2); value != 345 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	if value, err := buf.Get(3); value != 456 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	if value, err := buf.Get(4); value != 567 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	if value, err := buf.Get(5); value != 678 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}

	buf.Add(789)

	if value, err := buf.Get(0); err == nil {
		t.Errorf("expected error, got value %v", value)
	}
	if value, err := buf.Get(1); err == nil {
		t.Errorf("expected error, got value %v", value)
	}
	if value, err := buf.Get(2); value != 345 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	if value, err := buf.Get(3); value != 456 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	if value, err := buf.Get(4); value != 567 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	if value, err := buf.Get(5); value != 678 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}
	if value, err := buf.Get(6); value != 789 {
		t.Errorf("expected value, got value %v error %v", value, err)
	}

	// goleak.VerifyNone(t)
}
