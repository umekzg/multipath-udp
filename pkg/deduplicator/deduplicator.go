package deduplicator

type Deduplicator interface {
	FromSender(msg []byte) bool
	FromReceiver(msg []byte) bool
}
