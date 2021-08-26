package service

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
	"github.com/walkline/ToCloud9/shared/events"
)

type chatNatsListener struct {
	nc          *nats.Conn
	subs        []*nats.Subscription
	lbID        string
	broadcaster eBroadcaster.Broadcaster
}

func NewChatNatsListener(nc *nats.Conn, lbID string, broadcaster eBroadcaster.Broadcaster) Listener {
	return &chatNatsListener{
		nc:          nc,
		lbID:        lbID,
		broadcaster: broadcaster,
	}
}

func (c *chatNatsListener) Listen() error {
	sb, err := c.nc.Subscribe(events.ChatEventIncomingWhisper.SubjectName(c.lbID), func(msg *nats.Msg) {
		p := events.EventToReadGenericPayload{}
		err := json.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read LBEventCharacterLoggedIn event")
			return
		}

		chatMsg := events.ChatEventIncomingWhisperPayload{}
		err = json.Unmarshal(p.Payload, &chatMsg)
		if err != nil {
			log.Error().Err(err).Msg("can't read LBEventCharacterLoggedIn (payload part) event")
			return
		}

		c.broadcaster.NewIncomingWhisperEvent(&eBroadcaster.IncomingWhisperPayload{
			SenderGUID:   chatMsg.SenderGUID,
			SenderName:   chatMsg.SenderName,
			SenderRace:   chatMsg.SenderRace,
			ReceiverGUID: chatMsg.ReceiverGUID,
			ReceiverName: chatMsg.ReceiverName,
			Language:     chatMsg.Language,
			Msg:          chatMsg.Msg,
		})
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sb)
	return nil
}

func (c *chatNatsListener) Stop() error {
	return c.unsubscribe()
}

func (c *chatNatsListener) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
