package service

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/shared/events"
)

type CharactersListener struct {
	bg   BattleGroundService
	nc   *nats.Conn
	subs []*nats.Subscription
}

func NewCharactersListener(bgService BattleGroundService, nc *nats.Conn) *CharactersListener {
	return &CharactersListener{
		bg: bgService,
		nc: nc,
	}
}

func (c *CharactersListener) Listen() error {
	sb, err := c.nc.Subscribe(events.LBEventCharacterLoggedOut.SubjectName(), func(msg *nats.Msg) {
		loggedOutP := events.LBEventCharacterLoggedOutPayload{}
		_, err := events.Unmarshal(msg.Data, &loggedOutP)
		if err != nil {
			log.Error().Err(err).Msg("can't read LBEventCharacterLoggedOut (payload part) event")
			return
		}

		err = c.bg.PlayerBecomeOffline(context.Background(), loggedOutP.CharGUID, loggedOutP.RealmID)
		if err != nil {
			log.Error().Err(err).Msg("can't remove character in LBEventCharacterLoggedOut event")
			return
		}
	})
	if err != nil {
		c.unsubscribe()
		return err
	}

	c.subs = append(c.subs, sb)

	sb, err = c.nc.Subscribe(events.CharEventCharsDisconnectedUnhealthyLB.SubjectName(), func(msg *nats.Msg) {
		payload := events.CharEventCharsDisconnectedUnhealthyLBPayload{}
		_, err := events.Unmarshal(msg.Data, &payload)
		if err != nil {
			log.Error().Err(err).Msg("can't read LBEventCharacterLoggedOut (payload part) event")
			return
		}

		for _, char := range payload.CharactersGUID {
			err = c.bg.PlayerBecomeOffline(context.Background(), char, payload.RealmID)
			if err != nil {
				log.Error().Err(err).Msg("can't remove character in LBEventCharacterLoggedOut event")
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
