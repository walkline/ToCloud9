package service

import (
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/shared/events"
)

type serversRegistryNatsListener struct {
	nc          *nats.Conn
	subs        []*nats.Subscription
	broadcaster eBroadcaster.Broadcaster
}

func NewServersRegistryNatsListener(nc *nats.Conn, broadcaster eBroadcaster.Broadcaster) Listener {
	return &serversRegistryNatsListener{
		nc:          nc,
		broadcaster: broadcaster,
	}
}

func (c *serversRegistryNatsListener) Listen() error {
	sb, err := c.nc.Subscribe(events.ServerRegistryEventMatchmakingRemovedUnhealthy.SubjectName(), func(msg *nats.Msg) {
		payload := events.ServerRegistryEventMatchmakingRemovedUnhealthyPayload{}
		_, err := events.Unmarshal(msg.Data, &payload)
		if err != nil {
			log.Error().Err(err).Msg("can't read ServerRegistryEventMatchmakingRemovedUnhealthy event")
			return
		}

		c.broadcaster.NewMatchmakingServiceUnavailableEvent(&payload)
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sb)
	return nil
}

func (c *serversRegistryNatsListener) Stop() error {
	return c.unsubscribe()
}

func (c *serversRegistryNatsListener) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
