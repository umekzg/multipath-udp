package srt

import (
	"encoding/binary"
	"errors"
)

type ControlType uint16

type Subtype uint16

const (
	ControlTypeHandshake   ControlType = 0x0000
	ControlTypeKeepalive   ControlType = 0x0001
	ControlTypeAck         ControlType = 0x0002
	ControlTypeNak         ControlType = 0x0003
	ControlTypeShutdown    ControlType = 0x0005
	ControlTypeAckack      ControlType = 0x0006
	ControlTypeUserDefined ControlType = 0x7FFF
)

const (
	SubtypeMultipathAck Subtype = 0x0002
	SubtypeMultipathNak Subtype = 0x0003
)

type ControlPacket struct {
	RawPacket []byte
}

func NewMultipathAckControlPacket(pps uint32) *ControlPacket {
	pkt := make([]byte, 16)
	pkt[0] = 0xFF
	pkt[1] = 0xFF
	pkt[2] = 0x00
	pkt[3] = 0x02
	binary.BigEndian.PutUint32(pkt[4:8], pps)
	return &ControlPacket{RawPacket: pkt}
}

func (p *ControlPacket) Unmarshal(rawPacket []byte) error {
	if len(rawPacket) < 16 {
		return errors.New("packet too short")
	}
	p.RawPacket = rawPacket
	return nil
}

func (p ControlPacket) Marshal() []byte {
	return p.RawPacket
}

func (p ControlPacket) ControlType() ControlType {
	return ControlType(binary.BigEndian.Uint16(p.RawPacket[:2]) & 0x7FFF)
}

func (p ControlPacket) Subtype() Subtype {
	return Subtype(binary.BigEndian.Uint16(p.RawPacket[2:4]))
}

func (p ControlPacket) TypeSpecificInformation() uint32 {
	return binary.BigEndian.Uint32(p.RawPacket[4:8])
}

func (p ControlPacket) Timestamp() uint32 {
	return binary.BigEndian.Uint32(p.RawPacket[8:12])
}

func (p ControlPacket) DestinationSocketId() uint32 {
	return binary.BigEndian.Uint32(p.RawPacket[12:16])
}

func (p ControlPacket) HandshakeSocketId() uint32 {
	return binary.BigEndian.Uint32(p.RawPacket[40:44])
}

func NewNakControlPacket(from, to uint32) *ControlPacket {
	pkt := make([]byte, 24)
	pkt[0] = 0xFF
	pkt[1] = 0xFF
	pkt[2] = 0x00
	pkt[3] = 0x03
	binary.BigEndian.PutUint32(pkt[16:20], from)
	binary.BigEndian.PutUint32(pkt[20:24], from)
	return &ControlPacket{RawPacket: pkt}
}
