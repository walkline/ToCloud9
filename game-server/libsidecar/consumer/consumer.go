package consumer

import (
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/shared/events"
)

type Consumer interface {
	Start() error
	Stop() error
}

type natsConsumer struct {
	nc *nats.Conn
	// subs is all subscriptions list.
	subs []*nats.Subscription

	guildHandlersFabric          GuildHandlersFabric
	groupHandlersFabric          GroupHandlersFabric
	serverRegistryHandlersFabric ServerRegistryHandlerFabric

	queue queue.HandlersQueue

	realmID uint32
}

func NewNatsEventsConsumer(
	nc *nats.Conn,
	guildHandlersFabric GuildHandlersFabric,
	groupHandlersFabric GroupHandlersFabric,
	serverRegistryHandlersFabric ServerRegistryHandlerFabric,
	queue queue.HandlersQueue,
	realmID uint32,
) Consumer {
	return &natsConsumer{
		nc:                           nc,
		guildHandlersFabric:          guildHandlersFabric,
		groupHandlersFabric:          groupHandlersFabric,
		serverRegistryHandlersFabric: serverRegistryHandlersFabric,
		queue:                        queue,
		realmID:                      realmID,
	}
}

func (c *natsConsumer) Start() error {
	sub, err := c.nc.Subscribe(events.GuildEventMemberAdded.SubjectName(), func(msg *nats.Msg) {
		p := events.GuildEventMemberAddedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GuildEventMemberAdded (payload part) event")
			return
		}

		if p.RealmID != c.realmID {
			return
		}

		handler := c.guildHandlersFabric.GuildMemberAddedHandler(p.GuildID, p.MemberGUID)
		c.queue.Push(handler)
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GuildEventMemberKicked.SubjectName(), func(msg *nats.Msg) {
		p := events.GuildEventMemberKickedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GuildEventMemberKicked (payload part) event")
			return
		}

		if p.RealmID != c.realmID {
			return
		}

		handler := c.guildHandlersFabric.GuildMemberRemovedHandler(p.GuildID, p.MemberGUID)
		c.queue.Push(handler)
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GuildEventMemberLeft.SubjectName(), func(msg *nats.Msg) {
		p := events.GuildEventMemberLeftPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GuildEventMemberLeft (payload part) event")
			return
		}

		if p.RealmID != c.realmID {
			return
		}

		handler := c.guildHandlersFabric.GuildMemberLeftHandler(p.GuildID, p.MemberGUID)
		c.queue.Push(handler)
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupCreated.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventGroupCreatedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventGroupCreated (payload part) event")
			return
		}

		if p.RealmID != c.realmID {
			return
		}

		c.queue.Push(c.groupHandlersFabric.GroupCreated(&p))
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupMemberAdded.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventGroupMemberAddedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventGroupMemberAdded (payload part) event")
			return
		}

		if p.RealmID != c.realmID {
			return
		}

		c.queue.Push(c.groupHandlersFabric.GroupMemberAdded(&p))
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupMemberLeft.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventGroupMemberLeftPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventGroupMemberLeft (payload part) event")
			return
		}

		if p.RealmID != c.realmID {
			return
		}

		c.queue.Push(c.groupHandlersFabric.GroupMemberRemoved(&p))
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupDisband.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventGroupDisbandPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventGroupDisband (payload part) event")
			return
		}

		if p.RealmID != c.realmID {
			return
		}

		c.queue.Push(c.groupHandlersFabric.GroupDisbanded(&p))
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupLootTypeChanged.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventGroupLootTypeChangedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupLootTypeChanged (payload part) event")
			return
		}

		if p.RealmID != c.realmID {
			return
		}

		c.queue.Push(c.groupHandlersFabric.GroupLootTypeChanged(&p))
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupDifficultyChanged.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventGroupDifficultyChangedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventGroupDifficultyChanged (payload part) event")
			return
		}

		if p.RealmID != c.realmID {
			return
		}

		c.queue.Push(c.groupHandlersFabric.GroupDifficultyChanged(&p))
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupConvertedToRaid.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventGroupConvertedToRaidPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventGroupConvertedToRaid (payload part) event")
			return
		}

		if p.RealmID != c.realmID {
			return
		}

		c.queue.Push(c.groupHandlersFabric.GroupConvertedToRaid(&p))
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.ServerRegistryEventGSMapsReassigned.SubjectName(), func(msg *nats.Msg) {
		p := events.ServerRegistryEventGSMapsReassignedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read ServerRegistryEventGSMapsReassigned (payload part) event")
			return
		}

		c.queue.Push(c.serverRegistryHandlersFabric.GameServerMapsReassigned(&p))
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	return nil
}

func (c *natsConsumer) Stop() error {
	return c.unsubscribe()
}

func (c *natsConsumer) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
