package buffer

import (
	"testing"
	"time"
)

func TestReceiverBuffer(t *testing.T) {
	buf := NewReceiverBuffer(90000, 1*time.Second, 2*time.Second)

}
