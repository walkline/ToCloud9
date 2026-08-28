package session

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/apps/gateway/sockets"
)

type accountOwnershipTestSocket struct {
	read   chan *packet.Packet
	write  chan *packet.Packet
	closed chan struct{}
	once   sync.Once
}

func newAccountOwnershipTestSocket() *accountOwnershipTestSocket {
	return &accountOwnershipTestSocket{
		read: make(chan *packet.Packet), write: make(chan *packet.Packet, 1), closed: make(chan struct{}),
	}
}

func (s *accountOwnershipTestSocket) Close()                               { s.once.Do(func() { close(s.read); close(s.closed) }) }
func (*accountOwnershipTestSocket) ListenAndProcess(context.Context) error { return nil }
func (*accountOwnershipTestSocket) Address() string                        { return "test" }
func (s *accountOwnershipTestSocket) SendPacket(p *packet.Packet)          { s.write <- p }
func (s *accountOwnershipTestSocket) Send(p *packet.Writer) {
	s.write <- &packet.Packet{Opcode: p.Opcode, Data: p.Payload.Bytes()}
}
func (s *accountOwnershipTestSocket) ReadChannel() <-chan *packet.Packet  { return s.read }
func (s *accountOwnershipTestSocket) WriteChannel() chan<- *packet.Packet { return s.write }

type accountOwnershipTestCoordinator struct {
	mu       sync.Mutex
	claimed  bool
	released bool
	evict    func(context.Context) bool
}

func (o *accountOwnershipTestCoordinator) Register(_ string, evict func(context.Context) bool) func() {
	o.mu.Lock()
	o.evict = evict
	o.mu.Unlock()
	return func() {}
}
func (o *accountOwnershipTestCoordinator) Claim(context.Context, uint32, string) error {
	o.mu.Lock()
	o.claimed = true
	o.mu.Unlock()
	return nil
}
func (o *accountOwnershipTestCoordinator) Release(context.Context, uint32, string) error {
	o.mu.Lock()
	o.released = true
	o.mu.Unlock()
	return nil
}

func TestGameSessionClaimsAndReleasesAccountOwnership(t *testing.T) {
	socket := newAccountOwnershipTestSocket()
	coordinator := &accountOwnershipTestCoordinator{}
	logger := zerolog.Nop()
	session := NewGameSession(context.Background(), &logger, socket, 42, nil, GameSessionParams{
		AccountSessionOwnership: coordinator,
	})

	done := make(chan struct{})
	go func() {
		session.HandlePackets(context.Background())
		close(done)
	}()
	socket.Close()
	<-done

	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if !coordinator.claimed || !coordinator.released {
		t.Fatalf("ownership lifecycle incomplete: claimed=%v released=%v", coordinator.claimed, coordinator.released)
	}
}

func TestGameSessionEvictionWaitsForCompletedTeardown(t *testing.T) {
	socket := newAccountOwnershipTestSocket()
	coordinator := &accountOwnershipTestCoordinator{}
	logger := zerolog.Nop()
	session := NewGameSession(context.Background(), &logger, socket, 42, nil, GameSessionParams{
		AccountSessionOwnership: coordinator,
	})

	done := make(chan struct{})
	go func() {
		session.HandlePackets(context.Background())
		close(done)
	}()
	deadline := time.After(time.Second)
	var evict func(context.Context) bool
	for evict == nil {
		coordinator.mu.Lock()
		evict = coordinator.evict
		coordinator.mu.Unlock()
		select {
		case <-deadline:
			t.Fatal("session did not register its eviction callback")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if !evict(ctx) {
		t.Fatal("eviction did not confirm completed session teardown")
	}
	select {
	case <-socket.closed:
	case <-ctx.Done():
		t.Fatal("eviction did not close the previous game client")
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("evicted session did not stop")
	}
}

func TestCanceledWorldConnectClosesUnpublishedSocketBeforePlayerLogin(t *testing.T) {
	socket := newAccountOwnershipTestSocket()
	oldCreator := WorldSocketCreator
	WorldSocketCreator = func(*zerolog.Logger, string) (sockets.Socket, error) { return socket, nil }
	defer func() { WorldSocketCreator = oldCreator }()
	logger := zerolog.Nop()
	session := &GameSession{ctx: context.Background(), logger: &logger, authPacket: &packet.Packet{}}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := session.connectToGameServerWithAddress(ctx, 42, "world", nil)
		result <- err
	}()
	go func() { socket.read <- packet.NewWriter(packet.SMsgAuthChallenge).ToPacket() }()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled world connect = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled world connect did not return")
	}
	select {
	case <-socket.closed:
	case <-time.After(time.Second):
		t.Fatal("canceled world connect left its unpublished socket open")
	}
	if got := len(socket.write); got != 1 {
		t.Fatalf("socket write count = %d, want only auth and no player login", got)
	}
}

func TestAccountOwnershipFailsClosedWhenNATSFencingIsLost(t *testing.T) {
	ownership := NewNATSAccountSessionOwnership(nil, "gateway", 1)
	evicted := make(chan struct{})
	ownership.Register("token", func(context.Context) bool {
		close(evicted)
		return true
	})
	ownership.failClosed()

	select {
	case <-evicted:
	default:
		t.Fatal("loss of NATS fencing did not evict local sessions")
	}
	select {
	case <-ownership.Done():
	default:
		t.Fatal("loss of NATS fencing did not terminate ownership service")
	}
	if err := ownership.Claim(context.Background(), 1, "new-token"); err == nil {
		t.Fatal("ownership admitted a new account after losing NATS fencing")
	}
}

func TestAccountOwnershipUsesUniqueProcessEvictionIdentity(t *testing.T) {
	first := NewNATSAccountSessionOwnership(nil, "shared-gateway", 1)
	second := NewNATSAccountSessionOwnership(nil, "shared-gateway", 1)
	if first.instanceID == second.instanceID {
		t.Fatal("gateway processes reused an eviction identity")
	}
	decoded, err := hex.DecodeString(first.instanceID)
	if err != nil || len(decoded) != 16 {
		t.Fatalf("instance ID %q is not 128-bit lowercase hex: %v", first.instanceID, err)
	}
}
