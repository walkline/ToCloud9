package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	redis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func TestSessionOwnershipIntegrationTakeover(t *testing.T) {
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
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)

	const (
		realmID   = uint32(4294967294)
		accountID = uint32(987654321)
	)
	logger := zerolog.Nop()
	first := NewSessionOwnershipService(rdb, nc, &logger, "integration-gateway-1", realmID, 15*time.Second)
	second := NewSessionOwnershipService(rdb, nc, &logger, "integration-gateway-2", realmID, 15*time.Second)
	if err = first.Listen(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(first.Close)
	if err = second.Listen(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(second.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer rdb.Del(context.Background(),
		first.accountKey(accountID), first.evictionStreamKey(first.gatewayID),
		second.evictionStreamKey(second.gatewayID),
	)

	evicted := make(chan struct{})
	unregister := first.Register("token-1", func(context.Context) { close(evicted) })
	defer unregister()
	if err = first.ClaimAccount(ctx, accountID, "token-1"); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	takeoverStarted := time.Now()
	if err = second.ClaimAccount(ctx, accountID, "token-2"); err != nil {
		t.Fatalf("takeover claim: %v", err)
	}
	if elapsed := time.Since(takeoverStarted); elapsed > time.Second {
		t.Fatalf("live gateway takeover took %s", elapsed)
	}
	select {
	case <-evicted:
	case <-ctx.Done():
		t.Fatal("previous owner was not evicted")
	}
	if err = first.ReleaseAccount(ctx, accountID, "token-1"); err != nil {
		t.Fatal(err)
	}
	owner, err := rdb.Get(ctx, second.accountKey(accountID)).Result()
	if err != nil {
		t.Fatal(err)
	}
	if want := second.owner("token-2"); owner != want {
		t.Fatalf("owner = %q, want %q", owner, want)
	}
}

func TestSessionOwnershipIntegrationDurableEvictionWithoutNATS(t *testing.T) {
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
	firstNATS, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	secondNATS, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(secondNATS.Close)

	const (
		realmID   = uint32(4294967293)
		accountID = uint32(987654320)
	)
	logger := zerolog.Nop()
	first := NewSessionOwnershipService(rdb, firstNATS, &logger, "stream-gateway-1", realmID, 15*time.Second)
	second := NewSessionOwnershipService(rdb, secondNATS, &logger, "stream-gateway-2", realmID, 15*time.Second)
	if err = first.Listen(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(first.Close)
	if err = second.Listen(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(second.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer rdb.Del(context.Background(),
		first.accountKey(accountID), first.evictionStreamKey(first.gatewayID),
		second.evictionStreamKey(second.gatewayID),
	)

	evicted := make(chan struct{})
	unregister := first.Register("stream-token-1", func(context.Context) { close(evicted) })
	defer unregister()
	if err = first.ClaimAccount(ctx, accountID, "stream-token-1"); err != nil {
		t.Fatal(err)
	}
	firstNATS.Close() // force the durable Redis Stream path
	if err = second.ClaimAccount(ctx, accountID, "stream-token-2"); err != nil {
		t.Fatalf("durable takeover claim: %v", err)
	}
	select {
	case <-evicted:
	case <-ctx.Done():
		t.Fatal("Redis Stream did not evict the previous owner")
	}
}
