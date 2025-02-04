package service

import (
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/shared/events"
)

type ServersRegistryGSAddedConsumer interface {
	OnGameServerAdded(*events.ServerRegistryEventGSAddedPayload)
}

type ServersRegistryGSRemovedConsumer interface {
	OnGameServerRemoved(*events.ServerRegistryEventGSRemovedPayload)
}

type ServersRegistryListener struct {
	onGSAddedSubscriber   []ServersRegistryGSAddedConsumer
	onGSRemovedSubscriber []ServersRegistryGSRemovedConsumer

	nc       *nats.Conn
	natsSubs []*nats.Subscription
}

func NewServersRegistryListener(nc *nats.Conn, gsAddedConsumer []ServersRegistryGSAddedConsumer, gsRemovedConsumer []ServersRegistryGSRemovedConsumer) *ServersRegistryListener {
	return &ServersRegistryListener{
		nc:                    nc,
		onGSRemovedSubscriber: gsRemovedConsumer,
		onGSAddedSubscriber:   gsAddedConsumer,
	}
}

func (c *ServersRegistryListener) Listen() error {
	sb, err := c.nc.Subscribe(events.ServerRegistryEventGSAdded.SubjectName(), func(msg *nats.Msg) {
		payload := events.ServerRegistryEventGSAddedPayload{}
		_, err := events.Unmarshal(msg.Data, &payload)
		if err != nil {
			log.Error().Err(err).Msg("can't read ServerRegistryEventGSAdded (payload part) event")
			return
		}

		for _, gs := range c.onGSAddedSubscriber {
			gs.OnGameServerAdded(&payload)
		}
	})
	if err != nil {
		return err
	}

	c.natsSubs = append(c.natsSubs, sb)

	sb, err = c.nc.Subscribe(events.ServerRegistryEventGSRemoved.SubjectName(), func(msg *nats.Msg) {
		payload := events.ServerRegistryEventGSRemovedPayload{}
		_, err := events.Unmarshal(msg.Data, &payload)
		if err != nil {
			log.Error().Err(err).Msg("can't read ServerRegistryEventGSRemoved (payload part) event")
			return
		}

		for _, gs := range c.onGSRemovedSubscriber {
			gs.OnGameServerRemoved(&payload)
		}
	})
	if err != nil {
		return err
	}

	c.natsSubs = append(c.natsSubs, sb)

	return nil
}

func (c *ServersRegistryListener) Stop() error {
	return c.unsubscribe()
}

func (c *ServersRegistryListener) unsubscribe() error {
	for _, sub := range c.natsSubs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
