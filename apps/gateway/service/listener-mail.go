package service

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/shared/events"
)

type mailNatsListener struct {
	nc          *nats.Conn
	subs        []*nats.Subscription
	broadcaster eBroadcaster.Broadcaster
}

func NewMailNatsListener(nc *nats.Conn, broadcaster eBroadcaster.Broadcaster) Listener {
	return &mailNatsListener{
		nc:          nc,
		broadcaster: broadcaster,
	}
}

func (c *mailNatsListener) Listen() error {
	sb, err := c.nc.Subscribe(events.MailEventIncomingMail.SubjectName(), func(msg *nats.Msg) {
		p := events.EventToReadGenericPayload{}
		err := json.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read MailEventIncomingMail event")
			return
		}

		chatMsg := events.MailEventIncomingMailPayload{}
		err = json.Unmarshal(p.Payload, &chatMsg)
		if err != nil {
			log.Error().Err(err).Msg("can't read MailEventIncomingMail (payload part) event")
			return
		}

		c.broadcaster.NewIncomingMailEvent(&chatMsg)
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sb)
	return nil
}

func (c *mailNatsListener) Stop() error {
	return c.unsubscribe()
}

func (c *mailNatsListener) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
