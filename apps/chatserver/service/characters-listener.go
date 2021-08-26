package service

import (
	"context"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/chatserver/repo"
	"github.com/walkline/ToCloud9/apps/chatserver/sender"
	"github.com/walkline/ToCloud9/shared/events"
)

type CharactersListener struct {
	charRepo repo.CharactersRepo
	nc       *nats.Conn
	subs     []*nats.Subscription
}

func NewCharactersListener(charRepo repo.CharactersRepo, nc *nats.Conn) *CharactersListener {
	return &CharactersListener{
		charRepo: charRepo,
		nc:       nc,
	}
}

func (c *CharactersListener) Listen() error {
	sb, err := c.nc.Subscribe(events.LBEventCharacterLoggedIn.SubjectName(), func(msg *nats.Msg) {
		loggedInP := events.LBEventCharacterLoggedInPayload{}
		_, err := events.Unmarshal(msg.Data, &loggedInP)
		if err != nil {
			log.Error().Err(err).Msg("can't read LBEventCharacterLoggedIn (payload part) event")
			return
		}

		err = c.charRepo.AddCharacter(context.TODO(), &repo.Character{
			RealmID:        loggedInP.RealmID,
			LoadBalancerID: loggedInP.LoadBalancerID,
			GUID:           loggedInP.CharGUID,
			Name:           loggedInP.CharName,
			Race:           loggedInP.CharRace,
			MsgSender:      sender.NewMsgSenderNatsJSON(c.nc, loggedInP.LoadBalancerID),
		})

		if err != nil {
			log.Error().Err(err).Msg("can't add character in LBEventCharacterLoggedIn event")
			return
		}
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sb)

	sb, err = c.nc.Subscribe(events.LBEventCharacterLoggedOut.SubjectName(), func(msg *nats.Msg) {
		loggedOutP := events.LBEventCharacterLoggedOutPayload{}
		_, err := events.Unmarshal(msg.Data, &loggedOutP)
		if err != nil {
			log.Error().Err(err).Msg("can't read LBEventCharacterLoggedOut (payload part) event")
			return
		}

		err = c.charRepo.RemoveCharacter(context.TODO(), loggedOutP.RealmID, loggedOutP.CharGUID)
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
