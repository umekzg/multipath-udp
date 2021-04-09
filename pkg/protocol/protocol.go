package protocol

import (
	"encoding/binary"
	"time"
)

type PacketType uint8

const (
	HANDSHAKE PacketType = iota
	MISSING_SEQUENCE
	METER
	DATA
	UNKNOWN
)

// Packet represents the initial Packet between a sender and the receiver.
type HandshakePacket struct {
	SessionID string
}

type MissingSequencePacket struct {
	SequenceNumber uint64
}

type MeterPacket struct {
	PacketsReceived uint64
}

type DataPacket struct {
	SequenceNumber uint64
	Timestamp      time.Time
	Payload        []byte
}

func (p HandshakePacket) Serialize() []byte {
	data := []byte(p.SessionID)
	buf := make([]byte, len(data)+1)
	buf[0] = byte(HANDSHAKE)
	copy(buf[1:], data)
	return buf
}

func (p MissingSequencePacket) Serialize() []byte {
	buf := make([]byte, 9)
	buf[0] = byte(MISSING_SEQUENCE)
	binary.BigEndian.PutUint64(buf[1:], p.SequenceNumber)
	return buf
}

func (p MeterPacket) Serialize() []byte {
	buf := make([]byte, 9)
	buf[0] = byte(METER)
	binary.BigEndian.PutUint64(buf[1:], p.PacketsReceived)
	return buf
}

func (p DataPacket) Serialize() []byte {
	buf := make([]byte, 17+len(p.Payload))
	buf[0] = byte(DATA)
	binary.BigEndian.PutUint64(buf[1:], p.SequenceNumber)
	binary.BigEndian.PutUint64(buf[9:], uint64(p.Timestamp.UnixNano()))
	copy(buf[17:], p.Payload)
	return buf
}

func Deserialize(buf []byte) (PacketType, interface{}) {
	switch PacketType(buf[0]) {
	case HANDSHAKE:
		return HANDSHAKE, HandshakePacket{SessionID: string(buf[1:])}
	case MISSING_SEQUENCE:
		sequenceNumber := binary.BigEndian.Uint64(buf[1:9])
		return MISSING_SEQUENCE, MissingSequencePacket{SequenceNumber: sequenceNumber}
	case METER:
		packetsReceived := binary.BigEndian.Uint64(buf[1:9])
		return METER, MeterPacket{PacketsReceived: packetsReceived}
	case DATA:
		sequenceNumber := binary.BigEndian.Uint64(buf[1:9])
		timestamp := binary.BigEndian.Uint64(buf[9:17])

		return DATA, DataPacket{
			SequenceNumber: sequenceNumber,
			Timestamp:      time.Unix(0, int64(timestamp)),
			Payload:        buf[17:],
		}
	}
	return UNKNOWN, nil
}
