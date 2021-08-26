package sockets

import (
	"context"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
)

type Socket interface {
	Close()
	ListenAndProcess(ctx context.Context) error

	SendPacket(*packet.Packet)
	Send(*packet.Writer)

	ReadChannel() <-chan *packet.Packet
	WriteChannel() chan<- *packet.Packet
}
