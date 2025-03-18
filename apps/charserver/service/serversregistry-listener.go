package service

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

type ServersRegistryListener struct {
	charRepo repo.CharactersOnline
	nc       *nats.Conn
	subs     []*nats.Subscription
	producer events.CharactersServiceProducer
}

func NewServersRegistryListener(charRepo repo.CharactersOnline, producer events.CharactersServiceProducer, nc *nats.Conn) *ServersRegistryListener {
	return &ServersRegistryListener{
		charRepo: charRepo,
		nc:       nc,
		producer: producer,
	}
}

func (c *ServersRegistryListener) Listen() error {
	const charactersServiceGroup = "char_group"
	sb, err := c.nc.QueueSubscribe(events.ServerRegistryEventGWRemovedUnhealthy.SubjectName(), charactersServiceGroup, func(msg *nats.Msg) {
		payload := events.ServerRegistryEventGWRemovedUnhealthyPayload{}
		_, err := events.Unmarshal(msg.Data, &payload)
		if err != nil {
			log.Error().Err(err).Msg("can't read ServerRegistryEventGWRemovedUnhealthy (payload part) event")
			return
		}

		userIDs, err := c.charRepo.RemoveAllWithGatewayID(context.TODO(), payload.RealmID, payload.ID)
		if err != nil {
			log.Error().Err(err).Msg("can't delete characters in ServerRegistryEventGWRemovedUnhealthy event")
			return
		}

		if len(userIDs) > 0 {
			err = c.producer.CharsDisconnectedUnhealthyLB(&events.CharEventCharsDisconnectedUnhealthyGWPayload{
				RealmID:        payload.RealmID,
				GatewayID:      payload.ID,
				CharactersGUID: userIDs,
			})

			if err != nil {
				log.Error().Err(err).Msg("can't produce CharsDisconnectedUnhealthyLB event")
			}
		}
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sb)

	sb, err = c.nc.Subscribe(events.CharEventCharsDisconnectedUnhealthyGW.SubjectName(), func(msg *nats.Msg) {
		payload := events.CharEventCharsDisconnectedUnhealthyGWPayload{}
		_, err := events.Unmarshal(msg.Data, &payload)
		if err != nil {
			log.Error().Err(err).Msg("can't read CharEventCharsDisconnectedUnhealthyGW (payload part) event")
			return
		}

		_, err = c.charRepo.RemoveAllWithGatewayID(context.TODO(), payload.RealmID, payload.GatewayID)
		if err != nil {
			log.Error().Err(err).Msg("can't delete characters in CharEventCharsDisconnectedUnhealthyGW event")
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
