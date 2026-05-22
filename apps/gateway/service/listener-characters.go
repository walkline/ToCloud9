package service

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/shared/events"
)

type charactersNatsListener struct {
	nc          *nats.Conn
	subs        []*nats.Subscription
	broadcaster eBroadcaster.Broadcaster
}

func NewCharactersNatsListener(nc *nats.Conn, broadcaster eBroadcaster.Broadcaster) Listener {
	return &charactersNatsListener{
		nc:          nc,
		broadcaster: broadcaster,
	}
}

func (c *charactersNatsListener) Listen() error {
	if err := c.newSubscribe(events.CharEventArenaTeamInviteCreated, func() (interface{}, func()) {
		d := &events.CharEventArenaTeamInviteCreatedPayload{}
		return d, func() {
			c.broadcaster.NewArenaTeamInviteCreatedEvent(d)
		}
	}); err != nil {
		return err
	}

	return c.newSubscribe(events.CharEventArenaTeamNativeEvent, func() (interface{}, func()) {
		d := &events.CharEventArenaTeamNativeEventPayload{}
		return d, func() {
			c.broadcaster.NewArenaTeamNativeEvent(d)
		}
	})
}

func (c *charactersNatsListener) Stop() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}
	return nil
}

func (c *charactersNatsListener) newSubscribe(event events.CharactersServiceEvent, payloadAndHandler func() (interface{}, func())) error {
	sb, err := c.nc.Subscribe(event.SubjectName(), func(msg *nats.Msg) {
		p := events.EventToReadGenericPayload{}
		err := json.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msgf("can't read %v event", event)
			return
		}

		payload, handler := payloadAndHandler()
		err = json.Unmarshal(p.Payload, payload)
		if err != nil {
			log.Error().Err(err).Msgf("can't read %d (payload part) event", event)
			return
		}

		handler()
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sb)
	return nil
}
