package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/walkline/ToCloud9/shared/events"
)

const (
	accountEvictionTimeout = 24 * time.Second
	accountEvictionWorkers = 64
)

type AccountSessionOwnership interface {
	Register(token string, evict func(context.Context) bool) func()
	Claim(context.Context, uint32, string) error
	Release(context.Context, uint32, string) error
}

// NATSAccountSessionOwnership is the gateway side of the coordinator hosted
// by servers-registry. Gateways retain only local token-to-session callbacks;
// all authoritative ownership remains in servers-registry's existing Redis.
type NATSAccountSessionOwnership struct {
	nats       *nats.Conn
	gatewayID  string
	instanceID string
	realmID    uint32

	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.RWMutex
	sessions map[string]func(context.Context) bool

	dispatchMu  sync.Mutex
	closed      bool
	workers     chan struct{}
	wg          sync.WaitGroup
	sub         *nats.Subscription
	failed      chan struct{}
	failOnce    sync.Once
	closeOnce   sync.Once
	unavailable atomic.Bool
}

func NewNATSAccountSessionOwnership(nc *nats.Conn, gatewayID string, realmID uint32) *NATSAccountSessionOwnership {
	ctx, cancel := context.WithCancel(context.Background())
	return &NATSAccountSessionOwnership{
		nats: nc, gatewayID: gatewayID, instanceID: newAccountSessionToken(), realmID: realmID, ctx: ctx, cancel: cancel,
		sessions: make(map[string]func(context.Context) bool),
		workers:  make(chan struct{}, accountEvictionWorkers),
		failed:   make(chan struct{}),
	}
}

func (s *NATSAccountSessionOwnership) Listen() error {
	var err error
	s.sub, err = s.nats.Subscribe(events.AccountSessionEvictSubject(s.instanceID), s.handleEviction)
	if err != nil {
		return err
	}
	if err = s.nats.Flush(); err != nil {
		return err
	}
	status := s.nats.StatusChanged(nats.DISCONNECTED, nats.CLOSED)
	s.wg.Add(1)
	go s.monitorConnection(status)
	return nil
}

func (s *NATSAccountSessionOwnership) Close() {
	s.closeOnce.Do(func() {
		s.dispatchMu.Lock()
		s.closed = true
		if s.sub != nil {
			_ = s.sub.Unsubscribe()
		}
		s.dispatchMu.Unlock()
		s.cancel()
		s.wg.Wait()
	})
}

// Done closes after NATS connectivity is lost and every local session has been
// told to disconnect. The gateway must terminate before servers-registry can
// safely treat its missing eviction subscription as proof that it is gone.
func (s *NATSAccountSessionOwnership) Done() <-chan struct{} {
	return s.failed
}

func (s *NATSAccountSessionOwnership) Register(token string, evict func(context.Context) bool) func() {
	s.mu.Lock()
	s.sessions[token] = evict
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
	}
}

func (s *NATSAccountSessionOwnership) Claim(ctx context.Context, accountID uint32, token string) error {
	if s.unavailable.Load() {
		return errors.New("account session ownership is unavailable")
	}
	return s.request(ctx, events.AccountSessionClaimSubject, accountID, token)
}

func (s *NATSAccountSessionOwnership) Release(ctx context.Context, accountID uint32, token string) error {
	return s.request(ctx, events.AccountSessionReleaseSubject, accountID, token)
}

func (s *NATSAccountSessionOwnership) request(ctx context.Context, subject string, accountID uint32, token string) error {
	request := events.AccountSessionRequest{
		AccountID: accountID, RealmID: s.realmID, GatewayID: s.gatewayID,
		GatewayInstanceID: s.instanceID, Token: token,
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	message, err := s.nats.RequestWithContext(ctx, subject, payload)
	if err != nil {
		return fmt.Errorf("request account session ownership: %w", err)
	}
	var response events.AccountSessionResponse
	if err = json.Unmarshal(message.Data, &response); err != nil {
		return fmt.Errorf("decode account session ownership response: %w", err)
	}
	if !response.Success {
		return errors.New(response.Error)
	}
	return nil
}

func (s *NATSAccountSessionOwnership) handleEviction(message *nats.Msg) {
	s.dispatchMu.Lock()
	if s.closed {
		s.dispatchMu.Unlock()
		s.respondEviction(message, false, "gateway is stopping")
		return
	}
	select {
	case s.workers <- struct{}{}:
		s.wg.Add(1)
	default:
		s.dispatchMu.Unlock()
		s.respondEviction(message, false, "gateway eviction workers are busy")
		return
	}
	s.dispatchMu.Unlock()

	go func() {
		defer s.wg.Done()
		defer func() { <-s.workers }()
		var request events.AccountSessionEvictRequest
		if err := json.Unmarshal(message.Data, &request); err != nil {
			s.respondEviction(message, false, "invalid eviction request")
			return
		}
		s.mu.RLock()
		evict := s.sessions[request.Token]
		s.mu.RUnlock()
		if evict == nil {
			// The token is not active on this gateway. It is safe to confirm an
			// idempotent retry after release or a lost acknowledgement.
			s.respondEviction(message, true, "")
			return
		}
		ctx, cancel := context.WithTimeout(s.ctx, accountEvictionTimeout)
		confirmed := evict(ctx)
		cancel()
		if confirmed {
			s.respondEviction(message, true, "")
		} else {
			s.respondEviction(message, false, "session teardown timed out")
		}
	}()
}

func (s *NATSAccountSessionOwnership) respondEviction(message *nats.Msg, success bool, responseError string) {
	payload, err := json.Marshal(events.AccountSessionResponse{Success: success, Error: responseError})
	if err != nil {
		return
	}
	_ = message.Respond(payload)
}

func (s *NATSAccountSessionOwnership) monitorConnection(status <-chan nats.Status) {
	defer s.wg.Done()
	select {
	case <-s.ctx.Done():
		return
	case _, ok := <-status:
		if ok && s.ctx.Err() == nil {
			s.failClosed()
		}
	}
}

func (s *NATSAccountSessionOwnership) failClosed() {
	s.failOnce.Do(func() {
		s.unavailable.Store(true)
		s.mu.RLock()
		evictions := make([]func(context.Context) bool, 0, len(s.sessions))
		for _, evict := range s.sessions {
			evictions = append(evictions, evict)
		}
		s.mu.RUnlock()
		ctx, cancel := context.WithTimeout(context.Background(), accountEvictionTimeout)
		defer cancel()
		var wg sync.WaitGroup
		for _, evict := range evictions {
			wg.Add(1)
			go func(evict func(context.Context) bool) {
				defer wg.Done()
				evict(ctx)
			}(evict)
		}
		wg.Wait()
		close(s.failed)
	})
}

func newAccountSessionToken() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
