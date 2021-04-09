package srt

import (
	"encoding/binary"
	"fmt"
)

type Packet struct {
	IsControl      bool
	SequenceNumber uint32
	rawPacket      []byte
}

func Unmarshal(rawPacket []byte) (*Packet, error) {
	if len(rawPacket) < 4 {
		return nil, fmt.Errorf("packet size too small")
	}

	return &Packet{
		IsControl:      (rawPacket[0] & 0x80) > 0,
		SequenceNumber: binary.BigEndian.Uint32(rawPacket[:4]) & 0x7FFFFFFF,
		rawPacket:      rawPacket,
	}, nil
}

func (p Packet) Marshal() []byte {
	return p.rawPacket
}
