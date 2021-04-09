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

func (p Packet) ToControlPacket() (*ControlPacket, error) {
	if !p.IsControl {
		return nil, fmt.Errorf("not a control packet")
	}
	return &ControlPacket{
		ControlType: ControlType(binary.BigEndian.Uint16(p.rawPacket[:2]) & 0x7FFF),
		Subtype:     binary.BigEndian.Uint16(p.rawPacket[2:4]),
		rawPacket:   p.rawPacket,
	}, nil
}

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
	ControlType ControlType
	Subtype     uint16
	rawPacket   []byte
}
