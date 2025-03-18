package events

import (
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

type GroupEventInviteCreatedHandler interface {
	// GroupInviteCreatedEvent handles group invite created events.
	GroupInviteCreatedEvent(payload *GroupEventInviteCreatedPayload) error
}

type GroupEventGroupCreatedHandler interface {
	// GroupCreatedEvent handles group invite created events.
	GroupCreatedEvent(payload *GroupEventGroupCreatedPayload) error
}

type GroupEventGroupMemberOnlineStatusChangedHandler interface {
	// GroupMemberOnlineStatusChangedEvent handles group member online status changed events.
	GroupMemberOnlineStatusChangedEvent(payload *GroupEventGroupMemberOnlineStatusChangedPayload) error
}

type GroupEventGroupMemberLeftHandler interface {
	// GroupMemberLeftEvent handles group member left events.
	GroupMemberLeftEvent(payload *GroupEventGroupMemberLeftPayload) error
}

type GroupEventGroupDisbandHandler interface {
	// GroupDisbandEvent handles group disband events.
	GroupDisbandEvent(payload *GroupEventGroupDisbandPayload) error
}

type GroupEventMemberAddedHandler interface {
	// GroupMemberAddedEvent handles group member added events.
	GroupMemberAddedEvent(payload *GroupEventGroupMemberAddedPayload) error
}

type GroupEventLeaderChangedHandler interface {
	// GroupLeaderChangedEvent handles leader changed events.
	GroupLeaderChangedEvent(payload *GroupEventGroupLeaderChangedPayload) error
}

type GroupEventLootTypeChangedHandler interface {
	// GroupLootTypeChangedEvent handles loot type changed events.
	GroupLootTypeChangedEvent(payload *GroupEventGroupLootTypeChangedPayload) error
}

type GroupEventConvertedToRaidHandler interface {
	// GroupConvertedToRaidEvent handles group conversion to raid events.
	GroupConvertedToRaidEvent(payload *GroupEventGroupConvertedToRaidPayload) error
}

type GroupEventNewMessageHandler interface {
	// GroupChatMessageReceivedEvent handles new chat message event.
	GroupChatMessageReceivedEvent(payload *GroupEventNewMessagePayload) error
}

type GroupEventNewTargetIconHandler interface {
	// GroupTargetItemSetEvent handles new target icon event.
	GroupTargetItemSetEvent(payload *GroupEventNewTargetIconPayload) error
}

type GroupEventGroupDifficultyChangedHandler interface {
	// GroupDifficultyChangedEvent handles dungeon and raid difficulty changes.
	GroupDifficultyChangedEvent(payload *GroupEventGroupDifficultyChangedPayload) error
}

// GroupEventsConsumer listens to group events and handles events if there are handlers.
type GroupEventsConsumer interface {
	// Listen is non-blocking operation that listens to the group events.
	Listen() error

	// Stop stops listening to events.
	Stop() error
}

// NewGroupEventsConsumer creates new GroupEventsConsumer with given options.
// Need to provide at least one handler as option, otherwise it will not listen at all.
func NewGroupEventsConsumer(nc *nats.Conn, options ...GroupEventsConsumerOption) GroupEventsConsumer {
	params := &groupEventsConsumerParams{}
	for _, opt := range options {
		opt.apply(params)
	}
	return &groupEventsConsumerImpl{
		nc:                                    nc,
		inviteCreatedHandler:                  params.inviteCreatedHandler,
		groupCreatedHandler:                   params.groupCreatedHandler,
		groupMemberOnlineStatusChangedHandler: params.groupMemberOnlineStatusChangedHandler,
		groupMemberLeftHandler:                params.groupMemberLeftHandler,
		groupDisbandHandler:                   params.groupDisbandHandler,
		memberAddedHandler:                    params.memberAddedHandler,
		leaderChangedHandler:                  params.leaderChangedHandler,
		lootTypeChangedHandler:                params.lootTypeChangedHandler,
		convertedToRaidHandler:                params.convertedToRaidHandler,
		newMessageHandler:                     params.newMessageHandler,
		newTargetIconHandler:                  params.newTargetIconHandler,
		difficultyChangedHandler:              params.difficultyChangedHandler,
	}
}

// GroupEventsConsumerOption option to initialize group events consumer.
type GroupEventsConsumerOption interface {
	apply(*groupEventsConsumerParams)
}

// WithGroupEventConsumerInviteCreatedHandler creates group events consumer option with invite created handler.
// If not specified, listener will ignore this kind of events.
func WithGroupEventConsumerInviteCreatedHandler(h GroupEventInviteCreatedHandler) GroupEventsConsumerOption {
	return newFuncGroupEventsConsumerOption(func(params *groupEventsConsumerParams) {
		params.inviteCreatedHandler = h
	})
}

// WithGroupEventConsumerGroupCreatedHandler creates group events consumer option with group created handler.
// If not specified, listener will ignore this kind of events.
func WithGroupEventConsumerGroupCreatedHandler(h GroupEventGroupCreatedHandler) GroupEventsConsumerOption {
	return newFuncGroupEventsConsumerOption(func(params *groupEventsConsumerParams) {
		params.groupCreatedHandler = h
	})
}

// WithGroupEventConsumerGroupMemberOnlineStatusChangedHandler creates group events consumer option with group member
// online status changed.
// If not specified, listener will ignore this kind of events.
func WithGroupEventConsumerGroupMemberOnlineStatusChangedHandler(h GroupEventGroupMemberOnlineStatusChangedHandler) GroupEventsConsumerOption {
	return newFuncGroupEventsConsumerOption(func(params *groupEventsConsumerParams) {
		params.groupMemberOnlineStatusChangedHandler = h
	})
}

// WithGroupEventConsumerGroupMemberLeftHandler creates group events consumer option with group member left handler.
// If not specified, listener will ignore this kind of events.
func WithGroupEventConsumerGroupMemberLeftHandler(h GroupEventGroupMemberLeftHandler) GroupEventsConsumerOption {
	return newFuncGroupEventsConsumerOption(func(params *groupEventsConsumerParams) {
		params.groupMemberLeftHandler = h
	})
}

// WithGroupEventConsumerGroupDisbandHandler creates group events consumer option with group disband handler.
// If not specified, listener will ignore this kind of events.
func WithGroupEventConsumerGroupDisbandHandler(h GroupEventGroupDisbandHandler) GroupEventsConsumerOption {
	return newFuncGroupEventsConsumerOption(func(params *groupEventsConsumerParams) {
		params.groupDisbandHandler = h
	})
}

// WithGroupEventConsumerMemberAddedHandler creates group events consumer option with member added handler.
// If not specified, listener will ignore this kind of events.
func WithGroupEventConsumerMemberAddedHandler(h GroupEventMemberAddedHandler) GroupEventsConsumerOption {
	return newFuncGroupEventsConsumerOption(func(params *groupEventsConsumerParams) {
		params.memberAddedHandler = h
	})
}

// WithGroupEventConsumerLeaderChangedHandler creates group events consumer option with leader changed handler.
// If not specified, listener will ignore this kind of events.
func WithGroupEventConsumerLeaderChangedHandler(h GroupEventLeaderChangedHandler) GroupEventsConsumerOption {
	return newFuncGroupEventsConsumerOption(func(params *groupEventsConsumerParams) {
		params.leaderChangedHandler = h
	})
}

// WithGroupEventConsumerLootTypeChangedHandler creates group events consumer option with loot type changed handler.
// If not specified, listener will ignore this kind of events.
func WithGroupEventConsumerLootTypeChangedHandler(h GroupEventLootTypeChangedHandler) GroupEventsConsumerOption {
	return newFuncGroupEventsConsumerOption(func(params *groupEventsConsumerParams) {
		params.lootTypeChangedHandler = h
	})
}

// WithGroupEventConsumerConvertedToRaidHandler creates group events consumer option with converted to raid handler.
// If not specified, listener will ignore this kind of events.
func WithGroupEventConsumerConvertedToRaidHandler(h GroupEventConvertedToRaidHandler) GroupEventsConsumerOption {
	return newFuncGroupEventsConsumerOption(func(params *groupEventsConsumerParams) {
		params.convertedToRaidHandler = h
	})
}

// WithGroupEventNewChatMessageHandler creates group events consumer option with new chat message handler.
// If not specified, listener will ignore this kind of events.
func WithGroupEventNewChatMessageHandler(h GroupEventNewMessageHandler) GroupEventsConsumerOption {
	return newFuncGroupEventsConsumerOption(func(params *groupEventsConsumerParams) {
		params.newMessageHandler = h
	})
}

// WithGroupEventNewTargetIconHandler creates group events consumer option with new target icon handler.
// If not specified, listener will ignore this kind of events.
func WithGroupEventNewTargetIconHandler(h GroupEventNewTargetIconHandler) GroupEventsConsumerOption {
	return newFuncGroupEventsConsumerOption(func(params *groupEventsConsumerParams) {
		params.newTargetIconHandler = h
	})
}

// WithGroupDifficultyChangedHandler creates group events consumer option with difficulty changes handler.
// If not specified, listener will ignore this kind of events.
func WithGroupDifficultyChangedHandler(h GroupEventGroupDifficultyChangedHandler) GroupEventsConsumerOption {
	return newFuncGroupEventsConsumerOption(func(params *groupEventsConsumerParams) {
		params.difficultyChangedHandler = h
	})
}

// funcGatewayConsumerOption wraps a function that modifies funcGatewayConsumerOption into an
// implementation of the GatewayConsumerOption interface.
type funcGroupEventsConsumerOption struct {
	f func(*groupEventsConsumerParams)
}

func (f *funcGroupEventsConsumerOption) apply(do *groupEventsConsumerParams) {
	f.f(do)
}

func newFuncGroupEventsConsumerOption(f func(*groupEventsConsumerParams)) *funcGroupEventsConsumerOption {
	return &funcGroupEventsConsumerOption{
		f: f,
	}
}

// groupEventsConsumerParams list of all possible parameters of GroupEventsConsumer.
type groupEventsConsumerParams struct {
	inviteCreatedHandler                  GroupEventInviteCreatedHandler
	groupCreatedHandler                   GroupEventGroupCreatedHandler
	groupMemberOnlineStatusChangedHandler GroupEventGroupMemberOnlineStatusChangedHandler
	groupMemberLeftHandler                GroupEventGroupMemberLeftHandler
	groupDisbandHandler                   GroupEventGroupDisbandHandler
	memberAddedHandler                    GroupEventMemberAddedHandler
	leaderChangedHandler                  GroupEventLeaderChangedHandler
	lootTypeChangedHandler                GroupEventLootTypeChangedHandler
	convertedToRaidHandler                GroupEventConvertedToRaidHandler
	newMessageHandler                     GroupEventNewMessageHandler
	newTargetIconHandler                  GroupEventNewTargetIconHandler
	difficultyChangedHandler              GroupEventGroupDifficultyChangedHandler
}

// groupEventsConsumerImpl implementation of GroupEventsConsumer.
type groupEventsConsumerImpl struct {
	nc *nats.Conn
	// subs is all subscriptions list.
	subs []*nats.Subscription

	inviteCreatedHandler                  GroupEventInviteCreatedHandler
	groupCreatedHandler                   GroupEventGroupCreatedHandler
	groupMemberOnlineStatusChangedHandler GroupEventGroupMemberOnlineStatusChangedHandler
	groupMemberLeftHandler                GroupEventGroupMemberLeftHandler
	groupDisbandHandler                   GroupEventGroupDisbandHandler
	memberAddedHandler                    GroupEventMemberAddedHandler
	leaderChangedHandler                  GroupEventLeaderChangedHandler
	lootTypeChangedHandler                GroupEventLootTypeChangedHandler
	convertedToRaidHandler                GroupEventConvertedToRaidHandler
	newMessageHandler                     GroupEventNewMessageHandler
	newTargetIconHandler                  GroupEventNewTargetIconHandler
	difficultyChangedHandler              GroupEventGroupDifficultyChangedHandler
}

// Listen is non-blocking operation that listens to gateway events.
func (c *groupEventsConsumerImpl) Listen() error {
	if c.inviteCreatedHandler != nil {
		sub, err := c.nc.Subscribe(GroupEventInviteCreated.SubjectName(), func(msg *nats.Msg) {
			payload := GroupEventInviteCreatedPayload{}
			_, err := Unmarshal(msg.Data, &payload)
			if err != nil {
				log.Error().Err(err).Msg("can't read GroupEventInviteCreated (payload part) event")
				return
			}

			err = c.inviteCreatedHandler.GroupInviteCreatedEvent(&payload)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GroupEventInviteCreated event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.groupCreatedHandler != nil {
		sub, err := c.nc.Subscribe(GroupEventGroupCreated.SubjectName(), func(msg *nats.Msg) {
			payload := GroupEventGroupCreatedPayload{}
			_, err := Unmarshal(msg.Data, &payload)
			if err != nil {
				log.Error().Err(err).Msg("can't read GroupEventGroupCreated (payload part) event")
				return
			}

			err = c.groupCreatedHandler.GroupCreatedEvent(&payload)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GroupEventGroupCreated event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.groupMemberOnlineStatusChangedHandler != nil {
		sub, err := c.nc.Subscribe(GroupEventGroupMemberOnlineStatusChanged.SubjectName(), func(msg *nats.Msg) {
			payload := GroupEventGroupMemberOnlineStatusChangedPayload{}
			_, err := Unmarshal(msg.Data, &payload)
			if err != nil {
				log.Error().Err(err).Msg("can't read GroupEventGroupMemberOnlineStatusChanged (payload part) event")
				return
			}

			err = c.groupMemberOnlineStatusChangedHandler.GroupMemberOnlineStatusChangedEvent(&payload)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GroupEventGroupMemberOnlineStatusChanged event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.groupMemberLeftHandler != nil {
		sub, err := c.nc.Subscribe(GroupEventGroupMemberLeft.SubjectName(), func(msg *nats.Msg) {
			payload := GroupEventGroupMemberLeftPayload{}
			_, err := Unmarshal(msg.Data, &payload)
			if err != nil {
				log.Error().Err(err).Msg("can't read GroupEventGroupMemberLeft (payload part) event")
				return
			}

			err = c.groupMemberLeftHandler.GroupMemberLeftEvent(&payload)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GroupEventGroupMemberLeft event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.groupDisbandHandler != nil {
		sub, err := c.nc.Subscribe(GroupEventGroupDisband.SubjectName(), func(msg *nats.Msg) {
			payload := GroupEventGroupDisbandPayload{}
			_, err := Unmarshal(msg.Data, &payload)
			if err != nil {
				log.Error().Err(err).Msg("can't read GroupEventGroupDisband (payload part) event")
				return
			}

			err = c.groupDisbandHandler.GroupDisbandEvent(&payload)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GroupEventGroupDisband event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.memberAddedHandler != nil {
		sub, err := c.nc.Subscribe(GroupEventGroupMemberAdded.SubjectName(), func(msg *nats.Msg) {
			payload := GroupEventGroupMemberAddedPayload{}
			_, err := Unmarshal(msg.Data, &payload)
			if err != nil {
				log.Error().Err(err).Msg("can't read GroupEventGroupMemberAdded (payload part) event")
				return
			}

			err = c.memberAddedHandler.GroupMemberAddedEvent(&payload)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GroupEventGroupMemberAdded event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.leaderChangedHandler != nil {
		sub, err := c.nc.Subscribe(GroupEventGroupLeaderChanged.SubjectName(), func(msg *nats.Msg) {
			payload := GroupEventGroupLeaderChangedPayload{}
			_, err := Unmarshal(msg.Data, &payload)
			if err != nil {
				log.Error().Err(err).Msg("can't read GroupEventGroupLeaderChanged (payload part) event")
				return
			}

			err = c.leaderChangedHandler.GroupLeaderChangedEvent(&payload)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GroupEventGroupLeaderChanged event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.lootTypeChangedHandler != nil {
		sub, err := c.nc.Subscribe(GroupEventGroupLootTypeChanged.SubjectName(), func(msg *nats.Msg) {
			payload := GroupEventGroupLootTypeChangedPayload{}
			_, err := Unmarshal(msg.Data, &payload)
			if err != nil {
				log.Error().Err(err).Msg("can't read GroupEventGroupLootTypeChanged (payload part) event")
				return
			}

			err = c.lootTypeChangedHandler.GroupLootTypeChangedEvent(&payload)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GroupEventGroupLootTypeChanged event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.convertedToRaidHandler != nil {
		sub, err := c.nc.Subscribe(GroupEventGroupConvertedToRaid.SubjectName(), func(msg *nats.Msg) {
			payload := GroupEventGroupConvertedToRaidPayload{}
			_, err := Unmarshal(msg.Data, &payload)
			if err != nil {
				log.Error().Err(err).Msg("can't read GroupEventGroupConvertedToRaid (payload part) event")
				return
			}

			err = c.convertedToRaidHandler.GroupConvertedToRaidEvent(&payload)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GroupEventGroupConvertedToRaid event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.newMessageHandler != nil {
		sub, err := c.nc.Subscribe(GroupEventNewChatMessage.SubjectName(), func(msg *nats.Msg) {
			payload := GroupEventNewMessagePayload{}
			_, err := Unmarshal(msg.Data, &payload)
			if err != nil {
				log.Error().Err(err).Msg("can't read GroupEventNewChatMessage (payload part) event")
				return
			}

			err = c.newMessageHandler.GroupChatMessageReceivedEvent(&payload)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GroupEventNewChatMessage event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.newTargetIconHandler != nil {
		sub, err := c.nc.Subscribe(GroupEventNewTargetIcon.SubjectName(), func(msg *nats.Msg) {
			payload := GroupEventNewTargetIconPayload{}
			_, err := Unmarshal(msg.Data, &payload)
			if err != nil {
				log.Error().Err(err).Msg("can't read GroupEventNewTargetIconPayload (payload part) event")
				return
			}

			err = c.newTargetIconHandler.GroupTargetItemSetEvent(&payload)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GroupEventNewChatMessage event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.difficultyChangedHandler != nil {
		sub, err := c.nc.Subscribe(GroupEventGroupDifficultyChanged.SubjectName(), func(msg *nats.Msg) {
			payload := GroupEventGroupDifficultyChangedPayload{}
			_, err := Unmarshal(msg.Data, &payload)
			if err != nil {
				log.Error().Err(err).Msg("can't read GroupEventGroupDifficultyChangedPayload (payload part) event")
				return
			}

			err = c.difficultyChangedHandler.GroupDifficultyChangedEvent(&payload)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GroupEventGroupDifficultyChanged event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	return nil
}

// Stop stops listening to events.
func (c *groupEventsConsumerImpl) Stop() error {
	return c.unsubscribe()
}

func (c *groupEventsConsumerImpl) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
