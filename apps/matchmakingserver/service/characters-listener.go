package service

import (
	"context"
	"errors"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/shared/events"
)

type characterOfflineBattlegroundService interface {
	PlayerBecomeOffline(ctx context.Context, playerGUID uint64, realmID uint32) error
}

type characterOfflineLFGService interface {
	RemoveOfflinePlayer(ctx context.Context, realmID uint32, playerGUID uint64) error
}

type CharactersListener struct {
	bg   characterOfflineBattlegroundService
	lfg  characterOfflineLFGService
	nc   *nats.Conn
	subs []*nats.Subscription
	life *characterLifecycleTracker
}

func NewCharactersListener(bgService characterOfflineBattlegroundService, lfgService characterOfflineLFGService, nc *nats.Conn) *CharactersListener {
	return &CharactersListener{
		bg:   bgService,
		lfg:  lfgService,
		nc:   nc,
		life: newCharacterLifecycleTracker(),
	}
}

func (c *CharactersListener) Listen() error {
	sb, err := c.nc.Subscribe(events.GWEventCharacterLoggedIn.SubjectName(), func(msg *nats.Msg) {
		loggedInP := events.GWEventCharacterLoggedInPayload{}
		_, err := events.Unmarshal(msg.Data, &loggedInP)
		if err != nil {
			log.Error().Err(err).Msg("can't read GWEventCharacterLoggedIn (payload part) event")
			return
		}

		c.lifecycle().record(loggedInP.RealmID, loggedInP.CharGUID, loggedInP.EventTimeUnixNano)
	})
	if err != nil {
		c.unsubscribe()
		return err
	}

	c.subs = append(c.subs, sb)

	sb, err = c.nc.Subscribe(events.GWEventCharacterLoggedOut.SubjectName(), func(msg *nats.Msg) {
		loggedOutP := events.GWEventCharacterLoggedOutPayload{}
		_, err := events.Unmarshal(msg.Data, &loggedOutP)
		if err != nil {
			log.Error().Err(err).Msg("can't read GWEventCharacterLoggedOut (payload part) event")
			return
		}

		if !c.lifecycle().accept(loggedOutP.RealmID, loggedOutP.CharGUID, loggedOutP.EventTimeUnixNano) {
			log.Debug().
				Uint32("realmID", loggedOutP.RealmID).
				Uint64("playerGUID", loggedOutP.CharGUID).
				Uint64("eventTimeUnixNano", loggedOutP.EventTimeUnixNano).
				Msg("ignored stale matchmaking offline event")
			return
		}

		err = c.playerBecomeOffline(context.Background(), loggedOutP.RealmID, loggedOutP.CharGUID)
		if err != nil {
			log.Error().Err(err).Msg("can't remove character in GWEventCharacterLoggedOut event")
			return
		}
	})
	if err != nil {
		c.unsubscribe()
		return err
	}

	c.subs = append(c.subs, sb)

	sb, err = c.nc.Subscribe(events.CharEventCharsDisconnectedUnhealthyGW.SubjectName(), func(msg *nats.Msg) {
		payload := events.CharEventCharsDisconnectedUnhealthyGWPayload{}
		_, err := events.Unmarshal(msg.Data, &payload)
		if err != nil {
			log.Error().Err(err).Msg("can't read GWEventCharacterLoggedOut (payload part) event")
			return
		}

		for _, char := range payload.CharactersGUID {
			if !c.lifecycle().accept(payload.RealmID, char, payload.EventTimeUnixNano) {
				log.Debug().
					Uint32("realmID", payload.RealmID).
					Uint64("playerGUID", char).
					Str("gatewayID", payload.GatewayID).
					Uint64("eventTimeUnixNano", payload.EventTimeUnixNano).
					Msg("ignored stale matchmaking unhealthy-gateway offline event")
				continue
			}

			err = c.playerBecomeOffline(context.Background(), payload.RealmID, char)
			if err != nil {
				log.Error().Err(err).Msg("can't remove character in GWEventCharacterLoggedOut event")
			}
		}
	})
	if err != nil {
		c.unsubscribe()
		return err
	}

	c.subs = append(c.subs, sb)

	return nil
}

func (c *CharactersListener) playerBecomeOffline(ctx context.Context, realmID uint32, playerGUID uint64) error {
	var offlineErr error

	if c.bg != nil {
		if err := c.bg.PlayerBecomeOffline(ctx, playerGUID, realmID); err != nil {
			offlineErr = errors.Join(offlineErr, err)
		}
	}

	if c.lfg != nil {
		if err := c.lfg.RemoveOfflinePlayer(ctx, realmID, playerGUID); err != nil {
			if !errors.Is(err, ErrLFGNotFound) {
				offlineErr = errors.Join(offlineErr, err)
			}
		} else {
			log.Debug().
				Uint32("realmID", realmID).
				Uint64("playerGUID", playerGUID).
				Msg("removed offline character from LFG state")
		}
	}

	return offlineErr
}

func (c *CharactersListener) lifecycle() *characterLifecycleTracker {
	if c.life == nil {
		c.life = newCharacterLifecycleTracker()
	}
	return c.life
}

func (c *CharactersListener) Stop() error {
	return c.unsubscribe()
}

func (c *CharactersListener) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}

type characterLifecycleKey struct {
	realmID    uint32
	playerGUID uint64
}

type characterLifecycleTracker struct {
	mu         sync.Mutex
	eventTimes map[characterLifecycleKey]uint64
}

func newCharacterLifecycleTracker() *characterLifecycleTracker {
	return &characterLifecycleTracker{
		eventTimes: map[characterLifecycleKey]uint64{},
	}
}

func (t *characterLifecycleTracker) record(realmID uint32, playerGUID uint64, eventTimeUnixNano uint64) {
	if eventTimeUnixNano == 0 {
		return
	}
	key := characterLifecycleKey{realmID: realmID, playerGUID: playerGUID}
	t.mu.Lock()
	if t.eventTimes[key] < eventTimeUnixNano {
		t.eventTimes[key] = eventTimeUnixNano
	}
	t.mu.Unlock()
}

func (t *characterLifecycleTracker) accept(realmID uint32, playerGUID uint64, eventTimeUnixNano uint64) bool {
	if eventTimeUnixNano == 0 {
		return true
	}

	key := characterLifecycleKey{realmID: realmID, playerGUID: playerGUID}
	t.mu.Lock()
	defer t.mu.Unlock()

	if last := t.eventTimes[key]; last > eventTimeUnixNano {
		return false
	}
	t.eventTimes[key] = eventTimeUnixNano
	return true
}
