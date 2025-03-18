package service

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/chatserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

type ServersRegistryListener struct {
	charRepo repo.CharactersRepo
	nc       *nats.Conn
	subs     []*nats.Subscription
}

func NewServersRegistryListener(charRepo repo.CharactersRepo, nc *nats.Conn) *ServersRegistryListener {
	return &ServersRegistryListener{
		charRepo: charRepo,
		nc:       nc,
	}
}

func (c *ServersRegistryListener) Listen() error {
	sb, err := c.nc.Subscribe(events.ServerRegistryEventGWRemovedUnhealthy.SubjectName(), func(msg *nats.Msg) {
		payload := events.ServerRegistryEventGWRemovedUnhealthyPayload{}
		_, err := events.Unmarshal(msg.Data, &payload)
		if err != nil {
			log.Error().Err(err).Msg("can't read GWEventCharacterLoggedIn (payload part) event")
			return
		}

		err = c.charRepo.RemoveCharactersWithRealm(context.TODO(), payload.RealmID)
		if err != nil {
			log.Error().Err(err).Msg("can't add character in GWEventCharacterLoggedIn event")
			return
		}
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sb)

	return nil
}

func (c *ServersRegistryListener) Stop() error {
	return c.unsubscribe()
}

func (c *ServersRegistryListener) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
