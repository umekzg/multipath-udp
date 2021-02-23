package networking

import (
	lru "github.com/hashicorp/golang-lru"
	"github.com/segmentio/fasthash/fnv1a"
)

type PacketDeduplicator struct {
	caches map[string]*lru.Cache
}

func NewPacketDeduplicator() *PacketDeduplicator {
	return &PacketDeduplicator{
		caches: make(map[string]*lru.Cache),
	}
}

func (d *PacketDeduplicator) Receive(session string, msg []byte) bool {
	hash := fnv1a.HashBytes64(msg)

	cache, ok := d.caches[session]
	if ok {
		found, _ := cache.ContainsOrAdd(hash, true)
		return found
	}
	cache, err := lru.New(10000000)
	if err != nil {
		panic(err)
	}
	d.caches[session] = cache
	found, _ := cache.ContainsOrAdd(hash, true)
	return found
}
