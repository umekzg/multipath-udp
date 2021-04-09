package srt

import (
	"encoding/binary"
	"errors"
)

type DataPacket struct {
	rawPacket []byte
}

func (p *DataPacket) Unmarshal(rawPacket []byte) error {
	if len(rawPacket) < 16 {
		return errors.New("packet too short")
	}
	p.rawPacket = rawPacket
	return nil
}

func (p DataPacket) Marshal() []byte {
	return p.rawPacket
}

func (p DataPacket) SequenceNumber() uint32 {
	return binary.BigEndian.Uint32(p.rawPacket[:4]) & 0x7FFFFFFF
}

func (p DataPacket) Timestamp() uint32 {
	return binary.BigEndian.Uint32(p.rawPacket[8:12])
}

func (p DataPacket) DestinationSocketId() uint32 {
	return binary.BigEndian.Uint32(p.rawPacket[12:16])
}
