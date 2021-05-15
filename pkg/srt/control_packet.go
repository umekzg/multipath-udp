package srt

import (
	"encoding/binary"
	"errors"
	"time"
)

type PacketType uint16

type ControlType uint16

type Subtype uint16

const (
	PacketTypeControlPacket PacketType = 0x8000
	PacketTypeDataPacket    PacketType = 0x0000
)

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
	SubtypeMultipathAck       Subtype = 0x0002
	SubtypeMultipathNak       Subtype = 0x0003
	SubtypeMultipathKeepAlive Subtype = 0x0004
	SubtypeMultipathHandshake Subtype = 0x0005
)

type ControlPacket struct {
	RawPacket []byte
}

func NewMultipathAckControlPacket(pps uint32) *ControlPacket {
	pkt := make([]byte, 16)
	binary.BigEndian.PutUint16(pkt[0:2], uint16(PacketTypeControlPacket)|uint16(ControlTypeUserDefined))
	binary.BigEndian.PutUint16(pkt[2:4], uint16(SubtypeMultipathAck))
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

func NewNakRangeControlPacket(socketId, from, to uint32) *ControlPacket {
	if from == to {
		return NewNakSingleControlPacket(socketId, from)
	}
	pkt := make([]byte, 24)
	binary.BigEndian.PutUint16(pkt[0:2], uint16(PacketTypeControlPacket)|uint16(ControlTypeNak))
	binary.BigEndian.PutUint32(pkt[8:12], uint32(time.Now().UnixNano()/1000))
	binary.BigEndian.PutUint32(pkt[12:16], socketId)
	binary.BigEndian.PutUint32(pkt[16:20], from)
	binary.BigEndian.PutUint32(pkt[20:24], to|0x8000)
	return &ControlPacket{RawPacket: pkt}
}
func NewNakSingleControlPacket(socketId, seq uint32) *ControlPacket {
	pkt := make([]byte, 20)
	binary.BigEndian.PutUint16(pkt[0:2], uint16(PacketTypeControlPacket)|uint16(ControlTypeNak))
	binary.BigEndian.PutUint32(pkt[8:12], uint32(time.Now().UnixNano()/1000))
	binary.BigEndian.PutUint32(pkt[12:16], socketId)
	binary.BigEndian.PutUint32(pkt[16:20], seq)
	return &ControlPacket{RawPacket: pkt}
}

func NewMultipathNackControlPacket(severity, from, to uint32) *ControlPacket {
	pkt := make([]byte, 24)
	binary.BigEndian.PutUint16(pkt[0:2], uint16(PacketTypeControlPacket)|uint16(ControlTypeUserDefined))
	binary.BigEndian.PutUint16(pkt[2:4], uint16(SubtypeMultipathNak))
	binary.BigEndian.PutUint32(pkt[4:8], severity)
	binary.BigEndian.PutUint32(pkt[8:12], uint32(time.Now().UnixNano()/1000))
	binary.BigEndian.PutUint32(pkt[16:20], from)
	binary.BigEndian.PutUint32(pkt[20:24], to)
	return &ControlPacket{RawPacket: pkt}
}

func NewMultipathKeepAliveControlPacket() *ControlPacket {
	pkt := make([]byte, 16)
	binary.BigEndian.PutUint16(pkt[0:2], uint16(PacketTypeControlPacket)|uint16(ControlTypeUserDefined))
	binary.BigEndian.PutUint16(pkt[2:4], uint16(SubtypeMultipathNak))
	binary.BigEndian.PutUint32(pkt[8:12], uint32(time.Now().UnixNano()/1000))
	return &ControlPacket{RawPacket: pkt}
}

func NewMultipathHandshakeControlPacket(id uint32) *ControlPacket {
	pkt := make([]byte, 16)
	binary.BigEndian.PutUint16(pkt[0:2], uint16(PacketTypeControlPacket)|uint16(ControlTypeUserDefined))
	binary.BigEndian.PutUint16(pkt[2:4], uint16(SubtypeMultipathHandshake))
	binary.BigEndian.PutUint32(pkt[4:8], id)
	binary.BigEndian.PutUint32(pkt[8:12], uint32(time.Now().UnixNano()/1000))
	return &ControlPacket{RawPacket: pkt}
}

func NewKeepAlivePacket(dst uint32) *ControlPacket {
	pkt := make([]byte, 24)
	binary.BigEndian.PutUint16(pkt[0:2], uint16(PacketTypeControlPacket)|uint16(ControlTypeKeepalive))
	binary.BigEndian.PutUint32(pkt[8:12], uint32(time.Now().UnixNano()/1000))
	binary.BigEndian.PutUint32(pkt[12:16], dst)
	return &ControlPacket{RawPacket: pkt}
}
