package session

import (
	"context"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
)

type ownershipTestSocket struct {
	read   chan *packet.Packet
	write  chan *packet.Packet
	closed chan struct{}
	once   sync.Once
}

func newOwnershipTestSocket() *ownershipTestSocket {
	return &ownershipTestSocket{read: make(chan *packet.Packet), write: make(chan *packet.Packet, 1), closed: make(chan struct{})}
}

func (s *ownershipTestSocket) Close()                                 { s.once.Do(func() { close(s.read); close(s.closed) }) }
func (s *ownershipTestSocket) ListenAndProcess(context.Context) error { return nil }
func (s *ownershipTestSocket) Address() string                        { return "test" }
func (s *ownershipTestSocket) SendPacket(p *packet.Packet)            { s.write <- p }
func (s *ownershipTestSocket) Send(p *packet.Writer) {
	s.write <- &packet.Packet{Opcode: p.Opcode, Data: p.Payload.Bytes()}
}
func (s *ownershipTestSocket) ReadChannel() <-chan *packet.Packet  { return s.read }
func (s *ownershipTestSocket) WriteChannel() chan<- *packet.Packet { return s.write }

type ownershipTestCoordinator struct {
	mu           sync.Mutex
	registered   bool
	unregistered bool
	evict        func(context.Context)
}

func (o *ownershipTestCoordinator) Register(_ string, evict func(context.Context)) func() {
	o.mu.Lock()
	o.evict = evict
	o.registered = true
	o.mu.Unlock()
	return func() {
		o.mu.Lock()
		o.unregistered = true
		o.mu.Unlock()
	}
}
func (*ownershipTestCoordinator) ClaimCharacter(context.Context, uint64, string) error   { return nil }
func (*ownershipTestCoordinator) ReleaseCharacter(context.Context, uint64, string) error { return nil }

func TestGameSessionRegistersCharacterOwnershipCoordinator(t *testing.T) {
	socket := newOwnershipTestSocket()
	coordinator := &ownershipTestCoordinator{}
	logger := zerolog.Nop()
	session := NewGameSession(context.Background(), &logger, socket, 42, nil, GameSessionParams{SessionOwnership: coordinator})

	done := make(chan struct{})
	go func() {
		session.HandlePackets(context.Background())
		close(done)
	}()
	socket.Close()
	<-done

	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if !coordinator.registered || !coordinator.unregistered {
		t.Fatalf("ownership lifecycle incomplete: registered=%v unregistered=%v", coordinator.registered, coordinator.unregistered)
	}
}
