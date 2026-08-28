package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
)

type shutdownTestSocket struct {
	closed chan struct{}
	once   sync.Once
}

func newShutdownTestSocket() *shutdownTestSocket {
	return &shutdownTestSocket{closed: make(chan struct{})}
}

func (s *shutdownTestSocket) Close() { s.once.Do(func() { close(s.closed) }) }
func (s *shutdownTestSocket) ListenAndProcess(context.Context) error {
	<-s.closed
	return nil
}
func (*shutdownTestSocket) Address() string                     { return "test" }
func (*shutdownTestSocket) SendPacket(*packet.Packet)           {}
func (*shutdownTestSocket) Send(*packet.Writer)                 {}
func (*shutdownTestSocket) ReadChannel() <-chan *packet.Packet  { return nil }
func (*shutdownTestSocket) WriteChannel() chan<- *packet.Packet { return nil }

func TestActiveGatewayConnectionsCloseAndWait(t *testing.T) {
	connections := &activeGatewayConnections{}
	socket := newShutdownTestSocket()
	connections.Track(socket)
	go func() {
		defer connections.Done(socket)
		_ = socket.ListenAndProcess(context.Background())
	}()

	done := make(chan struct{})
	go func() {
		connections.CloseAndWait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("gateway shutdown did not close and join active connections")
	}
	select {
	case <-socket.closed:
	default:
		t.Fatal("gateway shutdown did not close the active socket")
	}
}
