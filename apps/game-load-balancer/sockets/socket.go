package sockets

import (
	"context"

	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
)

//go:generate mockery --name=Socket --output=socketmock --filename=socket.go
type Socket interface {
	Close()
	ListenAndProcess(ctx context.Context) error

	Address() string

	SendPacket(*packet.Packet)
	Send(*packet.Writer)

	ReadChannel() <-chan *packet.Packet
	WriteChannel() chan<- *packet.Packet
}
