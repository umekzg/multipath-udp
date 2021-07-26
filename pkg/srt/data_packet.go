package srt

import (
	"encoding/binary"
	"errors"
)

type DataPacket struct {
	RawPacket []byte
}

func (p *DataPacket) Unmarshal(rawPacket []byte) error {
	if len(rawPacket) < 16 {
		return errors.New("packet too short")
	}
	p.RawPacket = rawPacket
	return nil
}

func (p DataPacket) Marshal() []byte {
	return p.RawPacket
}

func (p DataPacket) SequenceNumber() uint32 {
	return binary.BigEndian.Uint32(p.RawPacket[:4]) & 0x7FFFFFFF
}

func (p DataPacket) Timestamp() uint32 {
	return binary.BigEndian.Uint32(p.RawPacket[8:12])
}

func (p DataPacket) DestinationSocketId() uint32 {
	return binary.BigEndian.Uint32(p.RawPacket[12:16])
}

func (p DataPacket) IsRetransmitted() bool {
	return (p.RawPacket[4] & 0b00001000) > 0
}
