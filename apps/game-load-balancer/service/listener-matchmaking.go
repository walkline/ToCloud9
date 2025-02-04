package service

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
	"github.com/walkline/ToCloud9/shared/events"
)

type matchmakingNatsListener struct {
	nc          *nats.Conn
	subs        []*nats.Subscription
	broadcaster eBroadcaster.Broadcaster
}

func NewMatchmakingNatsListener(nc *nats.Conn, broadcaster eBroadcaster.Broadcaster) Listener {
	return &matchmakingNatsListener{
		nc:          nc,
		broadcaster: broadcaster,
	}
}

func (c *matchmakingNatsListener) Listen() error {
	sb, err := c.nc.Subscribe(events.MatchmakingEventPlayersQueued.SubjectName(), func(msg *nats.Msg) {
		p := events.EventToReadGenericPayload{}
		err := json.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read MatchmakingEventPlayersQueued event")
			return
		}

		eventPayload := events.MatchmakingEventPlayersQueuedPayload{}
		err = json.Unmarshal(p.Payload, &eventPayload)
		if err != nil {
			log.Error().Err(err).Msg("can't read MatchmakingEventPlayersQueued (payload part) event")
			return
		}

		c.broadcaster.NewMatchmakingJoinedPVPQueueEvent(&eventPayload)
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sb)

	sb, err = c.nc.Subscribe(events.MatchmakingEventPlayersInvited.SubjectName(), func(msg *nats.Msg) {
		p := events.EventToReadGenericPayload{}
		err := json.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read MatchmakingEventPlayersInvited event")
			return
		}

		eventPayload := events.MatchmakingEventPlayersInvitedPayload{}
		err = json.Unmarshal(p.Payload, &eventPayload)
		if err != nil {
			log.Error().Err(err).Msg("can't read MatchmakingEventPlayersInvited (payload part) event")
			return
		}

		c.broadcaster.NewMatchmakingInvitedToBGOrArenaEvent(&eventPayload)
	})
	if err != nil {
		return err
	}

	sb, err = c.nc.Subscribe(events.MatchmakingEventPlayersInviteExpired.SubjectName(), func(msg *nats.Msg) {
		p := events.EventToReadGenericPayload{}
		err := json.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read MatchmakingEventPlayersInviteExpired event")
			return
		}

		eventPayload := events.MatchmakingEventPlayersInviteExpiredPayload{}
		err = json.Unmarshal(p.Payload, &eventPayload)
		if err != nil {
			log.Error().Err(err).Msg("can't read MatchmakingEventPlayersInviteExpired (payload part) event")
			return
		}

		c.broadcaster.NewMatchmakingInviteToBGOrArenaExpiredEvent(&eventPayload)
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sb)
	return nil
}

func (c *matchmakingNatsListener) Stop() error {
	return c.unsubscribe()
}

func (c *matchmakingNatsListener) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
