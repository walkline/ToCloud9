package service

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	redis "github.com/redis/go-redis/v9"

	gatewaySession "github.com/walkline/ToCloud9/apps/gateway/session"
	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
)

type accountOwnershipGatewayDirectory struct {
	mu       sync.RWMutex
	gateways map[uint32]map[string]repo.GatewayServer
}

func newAccountOwnershipGatewayDirectory() *accountOwnershipGatewayDirectory {
	return &accountOwnershipGatewayDirectory{gateways: make(map[uint32]map[string]repo.GatewayServer)}
}

func (d *accountOwnershipGatewayDirectory) set(realmID uint32, gatewayID string, active bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.gateways[realmID] == nil {
		d.gateways[realmID] = make(map[string]repo.GatewayServer)
	}
	if active {
		d.gateways[realmID][gatewayID] = repo.GatewayServer{ID: gatewayID, RealmID: realmID}
	} else {
		delete(d.gateways[realmID], gatewayID)
	}
}

func (*accountOwnershipGatewayDirectory) Register(context.Context, *repo.GatewayServer) (*repo.GatewayServer, error) {
	return nil, errors.New("not implemented")
}
func (*accountOwnershipGatewayDirectory) GatewayForRealm(context.Context, uint32) (*repo.GatewayServer, error) {
	return nil, errors.New("not implemented")
}
func (d *accountOwnershipGatewayDirectory) GatewaysForRealm(_ context.Context, realmID uint32) ([]repo.GatewayServer, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]repo.GatewayServer, 0, len(d.gateways[realmID]))
	for _, gateway := range d.gateways[realmID] {
		result = append(result, gateway)
	}
	return result, nil
}

func TestAccountSessionOwnershipIntegration(t *testing.T) {
	redisURL := os.Getenv("TC9_INTEGRATION_REDIS_URL")
	natsURL := os.Getenv("TC9_INTEGRATION_NATS_URL")
	if redisURL == "" || natsURL == "" {
		t.Skip("set TC9_INTEGRATION_REDIS_URL and TC9_INTEGRATION_NATS_URL to run")
	}
	redisOptions, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(redisOptions)
	t.Cleanup(func() { _ = rdb.Close() })
	registryNATS, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(registryNATS.Close)
	directory := newAccountOwnershipGatewayDirectory()
	coordinator := NewAccountSessionCoordinator(rdb, registryNATS, directory)
	if err = coordinator.Listen(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(coordinator.Close)

	newGateway := func(id string, realmID uint32) *gatewaySession.NATSAccountSessionOwnership {
		nc, connectErr := nats.Connect(natsURL)
		if connectErr != nil {
			t.Fatal(connectErr)
		}
		client := gatewaySession.NewNATSAccountSessionOwnership(nc, id, realmID)
		if listenErr := client.Listen(); listenErr != nil {
			t.Fatal(listenErr)
		}
		directory.set(realmID, id, true)
		t.Cleanup(func() { client.Close(); nc.Close() })
		return client
	}
	first := newGateway("ownership-gateway-1", 1)
	firstSibling := newGateway("ownership-gateway-1", 1)
	second := newGateway("ownership-gateway-2", 1)
	otherRealm := newGateway("ownership-gateway-realm-2", 2)
	deadGateway := newGateway("ownership-gateway-dead", 1)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	accountIDs := []uint32{4294967201, 4294967202, 4294967203, 4294967204}
	for _, accountID := range accountIDs {
		t.Cleanup(func() {
			_ = rdb.Del(context.Background(), accountSessionKey(accountID), accountSessionKey(accountID)+":pending").Err()
		})
	}

	t.Run("takeover evicts previous and fences its release", func(t *testing.T) {
		accountID := accountIDs[0]
		evicted := make(chan struct{})
		wrongProcessEvicted := make(chan struct{}, 1)
		unregister := first.Register("token-1", func(context.Context) bool { close(evicted); return true })
		defer unregister()
		unregisterSibling := firstSibling.Register("token-1", func(context.Context) bool {
			wrongProcessEvicted <- struct{}{}
			return true
		})
		defer unregisterSibling()
		if err := first.Claim(ctx, accountID, "token-1"); err != nil {
			t.Fatal(err)
		}
		if err := second.Claim(ctx, accountID, "token-2"); err != nil {
			t.Fatal(err)
		}
		select {
		case <-evicted:
		default:
			t.Fatal("previous account session was not evicted")
		}
		select {
		case <-wrongProcessEvicted:
			t.Fatal("a process sharing the registry gateway ID received the targeted eviction")
		case <-time.After(100 * time.Millisecond):
		}
		if err := first.Release(ctx, accountID, "token-1"); err != nil {
			t.Fatal(err)
		}
		owner, err := rdb.Get(ctx, accountSessionKey(accountID)).Result()
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parseAccountSessionOwner(owner)
		if err != nil || parsed.Token != "token-2" {
			t.Fatalf("owner after fenced old release = %+v, %v", parsed, err)
		}
	})

	t.Run("failed takeover preserves predecessor lineage", func(t *testing.T) {
		accountID := accountIDs[1]
		unregister := first.Register("lineage-token-1", func(context.Context) bool { return false })
		defer unregister()
		if err := first.Claim(ctx, accountID, "lineage-token-1"); err != nil {
			t.Fatal(err)
		}
		if err := second.Claim(ctx, accountID, "lineage-token-2"); err == nil {
			t.Fatal("takeover succeeded without confirmed predecessor teardown")
		}
		if err := otherRealm.Claim(ctx, accountID, "lineage-token-3"); err == nil {
			t.Fatal("later claimant bypassed the unconfirmed predecessor")
		}
		owner, err := rdb.Get(ctx, accountSessionKey(accountID)).Result()
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parseAccountSessionOwner(owner)
		if err != nil || parsed.Token != "lineage-token-1" {
			t.Fatalf("owner after failed takeovers = %+v, %v", parsed, err)
		}
	})

	t.Run("account ownership is global across realms", func(t *testing.T) {
		accountID := accountIDs[2]
		evicted := make(chan struct{})
		unregister := first.Register("global-token-1", func(context.Context) bool { close(evicted); return true })
		defer unregister()
		if err := first.Claim(ctx, accountID, "global-token-1"); err != nil {
			t.Fatal(err)
		}
		if err := otherRealm.Claim(ctx, accountID, "global-token-2"); err != nil {
			t.Fatal(err)
		}
		select {
		case <-evicted:
		default:
			t.Fatal("cross-realm login did not evict the previous account session")
		}
	})

	t.Run("inactive gateway cannot be used as an unsafe fencing signal", func(t *testing.T) {
		accountID := accountIDs[3]
		if err := deadGateway.Claim(ctx, accountID, "dead-token-1"); err != nil {
			t.Fatal(err)
		}
		deadGateway.Close()
		directory.set(1, "ownership-gateway-dead", false)
		if err := second.Claim(ctx, accountID, "dead-token-2"); err == nil {
			t.Fatal("claim bypassed an owner without confirmed teardown")
		}
	})
}
