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

		if !c.shouldConsumeGroupEvent(p.RealmID, groupMemberGUIDs(p.Members)) {
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

		if !c.shouldConsumeGroupEvent(p.RealmID, appendGroupGUID(p.OnlineMembers, p.MemberGUID)) {
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

		if !c.shouldConsumeGroupEvent(p.RealmID, appendGroupGUID(p.OnlineMembers, p.MemberGUID)) {
			return
		}

		c.queue.Push(c.groupHandlersFabric.GroupMemberRemoved(&p))
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupLeaderChanged.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventGroupLeaderChangedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventGroupLeaderChanged (payload part) event")
			return
		}

		if !c.shouldConsumeGroupEvent(p.RealmID, appendGroupGUIDs(p.OnlineMembers, p.PreviousLeader, p.NewLeader)) {
			return
		}

		c.queue.Push(c.groupHandlersFabric.GroupLeaderChanged(&p))
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

		if !c.shouldConsumeGroupEvent(p.RealmID, p.OnlineMembers) {
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

		if !c.shouldConsumeGroupEvent(p.RealmID, appendGroupGUID(p.OnlineMembers, p.NewLooterGUID)) {
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

		if !c.shouldConsumeGroupEvent(p.RealmID, appendGroupGUID(p.Receivers, p.Updater)) {
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

		if !c.shouldConsumeGroupEvent(p.RealmID, appendGroupGUID(p.OnlineMembers, p.Leader)) {
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

	sub, err = c.nc.Subscribe(events.GroupEventGroupReadyCheckStarted.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventReadyCheckStartedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventReadyCheckStarted event")
			return
		}
		if !c.shouldConsumeGroupEvent(p.RealmID, appendGroupGUID(p.Receivers, p.LeaderGUID)) {
			return
		}
		c.queue.Push(c.groupHandlersFabric.GroupReadyCheckStarted(&p))
	})
	if err != nil {
		return err
	}
	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupReadyCheckMemberState.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventReadyCheckMemberStatePayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventReadyCheckMemberState event")
			return
		}
		if !c.shouldConsumeGroupEvent(p.RealmID, p.Receivers) {
			return
		}
		c.queue.Push(c.groupHandlersFabric.GroupReadyCheckMemberState(&p))
	})
	if err != nil {
		return err
	}
	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupReadyCheckFinished.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventReadyCheckFinishedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventReadyCheckFinished event")
			return
		}
		if !c.shouldConsumeGroupEvent(p.RealmID, p.Receivers) {
			return
		}
		c.queue.Push(c.groupHandlersFabric.GroupReadyCheckFinished(&p))
	})
	if err != nil {
		return err
	}
	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupMemberSubGroupChanged.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventMemberSubGroupChangedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventMemberSubGroupChanged event")
			return
		}
		if !c.shouldConsumeGroupEvent(p.RealmID, appendGroupGUID(p.Receivers, p.MemberGUID)) {
			return
		}
		c.queue.Push(c.groupHandlersFabric.GroupMemberSubGroupChanged(&p))
	})
	if err != nil {
		return err
	}
	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupMemberFlagsChanged.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventMemberFlagsChangedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventMemberFlagsChanged event")
			return
		}
		if !c.shouldConsumeGroupEvent(p.RealmID, appendGroupGUID(p.Receivers, p.MemberGUID)) {
			return
		}
		c.queue.Push(c.groupHandlersFabric.GroupMemberFlagsChanged(&p))
	})
	if err != nil {
		return err
	}
	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupMemberStateChanged.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventMemberStateChangedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventMemberStateChanged event")
			return
		}
		if !c.shouldConsumeGroupEvent(p.RealmID, p.Receivers) {
			return
		}
		c.queue.Push(c.groupHandlersFabric.GroupMemberStateChanged(&p))
	})
	if err != nil {
		return err
	}
	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupMemberStatesChanged.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventMemberStatesChangedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventMemberStatesChanged event")
			return
		}
		if !c.shouldConsumeGroupEvent(p.RealmID, p.Receivers) {
			return
		}
		for _, state := range p.States {
			payload := groupMemberStateChangedPayloadFromBatch(&p, state)
			c.queue.Push(c.groupHandlersFabric.GroupMemberStateChanged(&payload))
		}
	})
	if err != nil {
		return err
	}
	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupInstanceResetRequest.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventInstanceResetRequestPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventInstanceResetRequest event")
			return
		}
		if !c.shouldConsumeGroupEvent(p.RealmID, appendGroupGUID(p.Receivers, p.PlayerGUID)) {
			return
		}
		c.queue.Push(c.groupHandlersFabric.GroupInstanceResetRequest(&p))
	})
	if err != nil {
		return err
	}
	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GroupEventGroupInstanceBindExtensionRequest.SubjectName(), func(msg *nats.Msg) {
		p := events.GroupEventInstanceBindExtensionRequestPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GroupEventInstanceBindExtensionRequest event")
			return
		}
		if !c.shouldConsumeGroupEvent(p.RealmID, appendGroupGUID(p.Receivers, p.PlayerGUID)) {
			return
		}
		c.queue.Push(c.groupHandlersFabric.GroupInstanceBindExtensionRequest(&p))
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

func groupMemberStateChangedPayloadFromBatch(batch *events.GroupEventMemberStatesChangedPayload, state events.GroupMemberStateUpdate) events.GroupEventMemberStateChangedPayload {
	return events.GroupEventMemberStateChangedPayload{
		ServiceID:           batch.ServiceID,
		RealmID:             batch.RealmID,
		GroupID:             batch.GroupID,
		SourceGatewayID:     batch.SourceGatewayID,
		SourceWorldserverID: batch.SourceWorldserverID,
		MemberGUID:          state.MemberGUID,
		Online:              state.Online,
		Level:               state.Level,
		Class:               state.Class,
		ZoneID:              state.ZoneID,
		MapID:               state.MapID,
		Health:              state.Health,
		MaxHealth:           state.MaxHealth,
		PowerType:           state.PowerType,
		Power:               state.Power,
		MaxPower:            state.MaxPower,
		AurasKnown:          state.AurasKnown,
		Auras:               state.Auras,
		Receivers:           batch.Receivers,
	}
}

func (c *natsConsumer) shouldConsumeGroupEvent(groupRealmID uint32, playerGUIDs []uint64) bool {
	for _, playerGUID := range playerGUIDs {
		if c.isLocalGroupPlayer(groupRealmID, playerGUID) {
			return true
		}
	}
	return false
}

func (c *natsConsumer) isLocalGroupPlayer(groupRealmID uint32, playerGUID uint64) bool {
	if playerGUID == 0 {
		return false
	}
	if playerGUID>>48 == 0 {
		if playerRealmID := uint32((playerGUID >> 32) & 0xffff); playerRealmID != 0 {
			return playerRealmID == c.realmID
		}
	}
	return groupRealmID == c.realmID
}

func groupMemberGUIDs(members []events.GroupMember) []uint64 {
	playerGUIDs := make([]uint64, 0, len(members))
	for _, member := range members {
		playerGUIDs = append(playerGUIDs, member.MemberGUID)
	}
	return playerGUIDs
}

func appendGroupGUID(playerGUIDs []uint64, playerGUID uint64) []uint64 {
	if playerGUID == 0 {
		return playerGUIDs
	}
	return appendGroupGUIDs(playerGUIDs, playerGUID)
}

func appendGroupGUIDs(playerGUIDs []uint64, extraGUIDs ...uint64) []uint64 {
	out := append([]uint64(nil), playerGUIDs...)
	for _, playerGUID := range extraGUIDs {
		if playerGUID != 0 {
			out = append(out, playerGUID)
		}
	}
	return out
}

func (c *natsConsumer) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
