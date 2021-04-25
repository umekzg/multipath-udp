package srt

import (
	"fmt"
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

	isControl := (rawPacket[0] & 0x80) > 0
	switch isControl {
	case false:
		packet = new(DataPacket)
	case true:
		packet = new(ControlPacket)
	}
	err = packet.Unmarshal(rawPacket)
	return packet, err
}
