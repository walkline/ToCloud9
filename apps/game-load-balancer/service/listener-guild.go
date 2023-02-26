package service

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
	"github.com/walkline/ToCloud9/shared/events"
)

type guildNatsListener struct {
	nc          *nats.Conn
	subs        []*nats.Subscription
	broadcaster eBroadcaster.Broadcaster
}

func NewGuildNatsListener(nc *nats.Conn, broadcaster eBroadcaster.Broadcaster) Listener {
	return &guildNatsListener{
		nc:          nc,
		broadcaster: broadcaster,
	}
}

func (g *guildNatsListener) Listen() error {
	err := g.newSubscribe(events.GuildEventInviteCreated, func() (interface{}, func()) {
		d := &events.GuildEventInviteCreatedPayload{}
		return d, func() {
			g.broadcaster.NewGuildInviteCreatedEvent(&eBroadcaster.GuildInviteCreatedPayload{
				RealmID:     d.RealmID,
				GuildID:     d.GuildID,
				GuildName:   d.GuildName,
				InviterGUID: d.InviterGUID,
				InviterName: d.InviterName,
				InviteeGUID: d.InviteeGUID,
				InviteeName: d.InviteeName,
			})
		}
	})
	if err != nil {
		return err
	}

	err = g.newSubscribe(events.GuildEventMemberPromote, func() (interface{}, func()) {
		d := &events.GuildEventMemberPromotePayload{}
		return d, func() {
			g.broadcaster.NewGuildMemberPromoteEvent(d)
		}
	})
	if err != nil {
		return err
	}

	err = g.newSubscribe(events.GuildEventMemberDemote, func() (interface{}, func()) {
		d := &events.GuildEventMemberDemotePayload{}
		return d, func() {
			g.broadcaster.NewGuildMemberDemoteEvent(d)
		}
	})
	if err != nil {
		return err
	}

	err = g.newSubscribe(events.GuildEventMemberAdded, func() (interface{}, func()) {
		d := &events.GuildEventMemberAddedPayload{}
		return d, func() {
			g.broadcaster.NewGuildMemberAddedEvent(d)
		}
	})
	if err != nil {
		return err
	}

	err = g.newSubscribe(events.GuildEventMemberLeft, func() (interface{}, func()) {
		d := &events.GuildEventMemberLeftPayload{}
		return d, func() {
			g.broadcaster.NewGuildMemberLeftEvent(d)
		}
	})
	if err != nil {
		return err
	}

	err = g.newSubscribe(events.GuildEventMemberKicked, func() (interface{}, func()) {
		d := &events.GuildEventMemberKickedPayload{}
		return d, func() {
			g.broadcaster.NewGuildMemberKickedEvent(d)
		}
	})
	if err != nil {
		return err
	}

	err = g.newSubscribe(events.GuildEventMOTDUpdated, func() (interface{}, func()) {
		d := &events.GuildEventMOTDUpdatedPayload{}
		return d, func() {
			g.broadcaster.NewGuildMOTDUpdatedEvent(d)
		}
	})
	if err != nil {
		return err
	}

	err = g.newSubscribe(events.GuildEventRankCreated, func() (interface{}, func()) {
		d := &events.GuildEventRankCreatedPayload{}
		return d, func() {
			g.broadcaster.NewGuildRankCreatedEvent(d)
		}
	})
	if err != nil {
		return err
	}

	err = g.newSubscribe(events.GuildEventRankUpdated, func() (interface{}, func()) {
		d := &events.GuildEventRankUpdatedPayload{}
		return d, func() {
			g.broadcaster.NewGuildRankUpdatedEvent(d)
		}
	})
	if err != nil {
		return err
	}

	err = g.newSubscribe(events.GuildEventRankDeleted, func() (interface{}, func()) {
		d := &events.GuildEventRankDeletedPayload{}
		return d, func() {
			g.broadcaster.NewGuildRankDeletedEvent(d)
		}
	})
	if err != nil {
		return err
	}

	err = g.newSubscribe(events.GuildEventNewMessage, func() (interface{}, func()) {
		d := &events.GuildEventNewMessagePayload{}
		return d, func() {
			g.broadcaster.NewGuildMessageEvent(d)
		}
	})
	if err != nil {
		return err
	}

	return nil
}

func (g *guildNatsListener) Stop() error {
	return g.unsubscribe()
}

func (g *guildNatsListener) unsubscribe() error {
	for _, sub := range g.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}

func (g *guildNatsListener) newSubscribe(event events.GuildServiceEvent, payloadAndHandler func() (interface{}, func())) error {
	sb, err := g.nc.Subscribe(event.SubjectName(), func(msg *nats.Msg) {
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

	g.subs = append(g.subs, sb)

	return nil
}
