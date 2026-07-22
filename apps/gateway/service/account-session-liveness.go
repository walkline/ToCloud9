package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
)

func NewGatewaySessionInstanceID() (string, error) {
	var suffix [16]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return "gw-" + hex.EncodeToString(suffix[:]), nil
}

// AccountSessionGatewayLiveness writes one database-backed heartbeat for the
// gateway, independent of the number of connected accounts.
type AccountSessionGatewayLiveness struct {
	client    pbChar.CharactersServiceClient
	logger    *zerolog.Logger
	gatewayID string
	realmID   uint32
	ttl       time.Duration
}

func NewAccountSessionGatewayLiveness(client pbChar.CharactersServiceClient, logger *zerolog.Logger, gatewayID string, realmID uint32, ttl time.Duration) *AccountSessionGatewayLiveness {
	return &AccountSessionGatewayLiveness{client: client, logger: logger, gatewayID: gatewayID, realmID: realmID, ttl: ttl}
}

func (l *AccountSessionGatewayLiveness) Heartbeat(ctx context.Context) error {
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := l.client.HeartbeatGatewaySession(callCtx, &pbChar.HeartbeatGatewaySessionRequest{
		GatewayID:       l.gatewayID,
		RealmID:         l.realmID,
		LivenessSeconds: uint32(l.ttl / time.Second),
	})
	return err
}

// Run refreshes liveness until cancellation. It returns before the last
// successful heartbeat can expire so the caller can terminate the gateway and
// prevent stale sessions from overlapping with a replacement owner.
func (l *AccountSessionGatewayLiveness) Run(ctx context.Context) error {
	interval := l.ttl / 3
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	lastSuccess := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := l.Heartbeat(ctx); err != nil {
				l.logger.Warn().Err(err).Msg("can't refresh gateway account-session liveness")
				if time.Since(lastSuccess) >= l.ttl-interval {
					return fmt.Errorf("gateway account-session liveness is about to expire: %w", err)
				}
				continue
			}
			lastSuccess = time.Now()
		}
	}
}
