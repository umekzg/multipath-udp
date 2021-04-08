package protocol

import (
	"reflect"
	"testing"
	"time"
)

func TestHandshake(t *testing.T) {
	h := HandshakePacket{SessionID: "mootest"}
	packetType, d := Deserialize(h.Serialize())
	if packetType != HANDSHAKE {
		t.Errorf("invalid packet type: %v", packetType)
	}
	if !reflect.DeepEqual(d, h) {
		t.Errorf("deserialization incorrect, got %v, want %v", d, h)
	}
}

func TestMissingSequence(t *testing.T) {
	h := MissingSequencePacket{SequenceNumber: 2378946}
	packetType, d := Deserialize(h.Serialize())
	if packetType != MISSING_SEQUENCE {
		t.Errorf("invalid packet type: %v", packetType)
	}
	if !reflect.DeepEqual(d, h) {
		t.Errorf("deserialization incorrect, got %v, want %v", d, h)
	}
}

func TestMeter(t *testing.T) {
	h := MeterPacket{PacketsReceived: 234985734985}
	packetType, d := Deserialize(h.Serialize())
	if packetType != METER {
		t.Errorf("invalid packet type: %v", packetType)
	}
	if !reflect.DeepEqual(d, h) {
		t.Errorf("deserialization incorrect, got %v, want %v", d, h)
	}
}

func TestData(t *testing.T) {
	now := time.Now()
	h := DataPacket{
		SequenceNumber: 23934857,
		Timestamp:      time.Unix(0, now.UnixNano()),
		Payload:        []byte("mooogit test payload"),
	}
	packetType, d := Deserialize(h.Serialize())
	if packetType != DATA {
		t.Errorf("invalid packet type: %v", packetType)
	}
	if !reflect.DeepEqual(d, h) {
		t.Errorf("deserialization incorrect, got %v, want %v", d, h)
	}
}
