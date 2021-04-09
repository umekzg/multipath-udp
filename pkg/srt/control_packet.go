package srt

import (
	"encoding/binary"
	"errors"
)

type ControlType uint16

const (
	ControlTypeHandshake ControlType = 0x0000
	ControlTypeKeepalive ControlType = 0x0001
	ControlTypeAck       ControlType = 0x0002
	ControlTypeNak       ControlType = 0x0003
	ControlTypeShutdown  ControlType = 0x0005
	ControlTypeAckack    ControlType = 0x0006
)

type ControlPacket struct {
	rawPacket []byte
}

func (p *ControlPacket) Unmarshal(rawPacket []byte) error {
	if len(rawPacket) < 16 {
		return errors.New("packet too short")
	}
	p.rawPacket = rawPacket
	return nil
}

func (p ControlPacket) Marshal() []byte {
	return p.rawPacket
}

func (p ControlPacket) ControlType() ControlType {
	return ControlType(binary.BigEndian.Uint16(p.rawPacket[:2]) & 0x7FFF)
}

func (p ControlPacket) Subtype() uint16 {
	return binary.BigEndian.Uint16(p.rawPacket[2:4])
}

func (p ControlPacket) Timestamp() uint32 {
	return binary.BigEndian.Uint32(p.rawPacket[8:12])
}

func (p ControlPacket) DestinationSocketId() uint32 {
	return binary.BigEndian.Uint32(p.rawPacket[12:16])
}

func (p ControlPacket) HandshakeSocketId() uint32 {
	return binary.BigEndian.Uint32(p.rawPacket[44:48])
}
