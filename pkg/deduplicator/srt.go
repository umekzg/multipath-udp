package deduplicator

import (
	"bytes"
	"encoding/binary"

	lru "github.com/hashicorp/golang-lru"
)

type SrtDeduplicator struct {
	cache *lru.Cache
}

func NewSrtDeduplicator(size int) (*SrtDeduplicator, error) {
	cache, err := lru.New(size)
	if err != nil {
		return nil, err
	}
	return &SrtDeduplicator{cache: cache}, nil
}

func (d *SrtDeduplicator) isControlPacket(msg []byte) bool {
	return (msg[0] & (1 << 7)) != 0
}

func (d *SrtDeduplicator) getPacketSequenceNumber(msg []byte) uint32 {
	nowBuffer := bytes.NewReader(msg[:4])
	var nowVar uint32
	binary.Read(nowBuffer, binary.BigEndian, &nowVar)
	return nowVar
}

func (d *SrtDeduplicator) FromSender(msg []byte) bool {
	if d.isControlPacket(msg) {
		return true
	}
	seq := d.getPacketSequenceNumber(msg)
	found, _ := d.cache.ContainsOrAdd(seq, true)
	return !found
}

func (d *SrtDeduplicator) FromReceiver(msg []byte) bool {
	if d.isControlPacket(msg) {
		return true
	}
	seq := d.getPacketSequenceNumber(msg)
	found, _ := d.cache.ContainsOrAdd(seq, true)
	return !found
}
