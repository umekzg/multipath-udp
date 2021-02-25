package protocol

import (
	"bytes"
	"reflect"
	"testing"
)

func TestHandshake(t *testing.T) {
	h := NewHandshake("test", []byte{1, 2, 3})

	serialized, err := h.Serialize()
	if err != nil {
		t.Errorf("received serialization error %v", err)
	}

	deserialized, err := Deserialize(serialized)
	if err != nil {
		t.Errorf("received deserialization error %v", err)
	}

	if !reflect.DeepEqual(h, deserialized) {
		t.Errorf("didn't deserialize to equal values %v vs %v", deserialized, h)
	}

	reserialized, err := deserialized.Serialize()
	if err != nil {
		t.Errorf("received serialization error %v", err)
	}

	if !bytes.Equal(reserialized, serialized) {
		t.Errorf("didn't stably serialize to equal values %v vs %v", serialized, reserialized)
	}
}
