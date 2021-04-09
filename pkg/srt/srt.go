package srt

import (
	"fmt"
)

type PacketType bool

const (
	PacketTypeControl PacketType = true
	PacketTypeData    PacketType = false
)

type Packet interface {
	Timestamp() uint32
	DestinationSocketId() uint32

	Marshal() []byte
	Unmarshal(rawPacket []byte) error
}

func Unmarshal(rawPacket []byte) (packet Packet, err error) {
	if len(rawPacket) < 16 {
		return nil, fmt.Errorf("packet size too small")
	}

	packetType := PacketType((rawPacket[0] & 0x80) > 0)
	switch packetType {
	case PacketTypeData:
		packet = new(DataPacket)
	case PacketTypeControl:
		packet = new(ControlPacket)
	}
	err = packet.Unmarshal(rawPacket)
	return packet, err
}
