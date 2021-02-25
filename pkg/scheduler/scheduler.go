package scheduler

import (
	"net"
)

type Scheduler interface {
	SenderInit(receiverAddr *net.UDPAddr)
	ReceiverInit(port int)
	Schedule(senders []string, msg []byte) []string
	Receive(sender string, msg []byte)
}
