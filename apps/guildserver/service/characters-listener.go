package service

import (
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/shared/events"
)

// CharactersListener handles characters service events. A gateway that dies
// can't publish per-session logged-out events, so without this listener the
// members it hosted stay online in the guilds cache until they log in and
// out again through a healthy gateway.
type CharactersListener struct {
	handlers []events.GWCharacterLoggedOutHandler
	nc       *nats.Conn
	subs     []*nats.Subscription
}

func NewCharactersListener(nc *nats.Conn, handlers ...events.GWCharacterLoggedOutHandler) *CharactersListener {
	return &CharactersListener{
		handlers: handlers,
		nc:       nc,
	}
}

func (c *CharactersListener) Listen() error {
	sb, err := c.nc.Subscribe(events.CharEventCharsDisconnectedUnhealthyGW.SubjectName(), func(msg *nats.Msg) {
		payload := events.CharEventCharsDisconnectedUnhealthyGWPayload{}
		_, err := events.Unmarshal(msg.Data, &payload)
		if err != nil {
			log.Error().Err(err).Msg("can't read CharEventCharsDisconnectedUnhealthyGW (payload part) event")
			return
		}

		c.handleDisconnectedChars(&payload)
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sb)

	return nil
}

func (c *CharactersListener) Stop() error {
	return c.unsubscribe()
}

func (c *CharactersListener) handleDisconnectedChars(payload *events.CharEventCharsDisconnectedUnhealthyGWPayload) {
	for _, char := range payload.CharactersGUID {
		loggedOut := events.GWEventCharacterLoggedOutPayload{
			RealmID:   payload.RealmID,
			GatewayID: payload.GatewayID,
			CharGUID:  char,
		}

		for _, h := range c.handlers {
			if err := h.HandleCharacterLoggedOut(loggedOut); err != nil {
				log.Error().Err(err).Uint64("char", char).Msg("can't handle logged out for character of unhealthy gateway")
			}
		}
	}
}

func (c *CharactersListener) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}
	return nil
}
