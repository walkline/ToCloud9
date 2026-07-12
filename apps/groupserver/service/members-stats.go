package service

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/apps/groupserver"
	"github.com/walkline/ToCloud9/apps/groupserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

// MembersStatsCollector aggregates characters updates coming from gateways, keeps only
// characters that are in a group and periodically publishes batched per-group
// members updates events that gateways use to build party member stats packets.
type MembersStatsCollector struct {
	logger   *zerolog.Logger
	repo     repo.GroupsRepo
	producer events.GroupServiceProducer

	flushInterval time.Duration

	mu      sync.Mutex
	pending map[uint32]map[uint64]events.CharacterUpdate
}

func NewMembersStatsCollector(logger *zerolog.Logger, r repo.GroupsRepo, producer events.GroupServiceProducer, flushInterval time.Duration) *MembersStatsCollector {
	if logger == nil {
		nop := zerolog.Nop()
		logger = &nop
	}

	return &MembersStatsCollector{
		logger:        logger,
		repo:          r,
		producer:      producer,
		flushInterval: flushInterval,
		pending:       map[uint32]map[uint64]events.CharacterUpdate{},
	}
}

// HandleCharactersUpdates implements events.GWCharactersUpdatesHandler.
func (c *MembersStatsCollector) HandleCharactersUpdates(payload events.GWEventCharactersUpdatesPayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	realmPending := c.pending[payload.RealmID]
	if realmPending == nil {
		realmPending = map[uint64]events.CharacterUpdate{}
		c.pending[payload.RealmID] = realmPending
	}

	for _, upd := range payload.Updates {
		if upd == nil {
			continue
		}

		merged, ok := realmPending[upd.ID]
		if !ok {
			merged = events.CharacterUpdate{ID: upd.ID}
		}
		mergeCharacterUpdate(&merged, upd)
		realmPending[upd.ID] = merged
	}

	return nil
}

// HandleCharacterLoggedOut implements events.GWCharacterLoggedOutHandler: drops
// pending updates of a character that logged out, so a late flush doesn't mark
// the member as online again right after clients were told it went offline.
func (c *MembersStatsCollector) HandleCharacterLoggedOut(payload events.GWEventCharacterLoggedOutPayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if realmPending := c.pending[payload.RealmID]; realmPending != nil {
		delete(realmPending, payload.CharGUID)
	}

	return nil
}

// Run flushes collected updates every flushInterval until ctx is cancelled.
func (c *MembersStatsCollector) Run(ctx context.Context) {
	t := time.NewTicker(c.flushInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.flush(ctx)
		}
	}
}

func (c *MembersStatsCollector) flush(ctx context.Context) {
	c.mu.Lock()
	pending := c.pending
	c.pending = map[uint32]map[uint64]events.CharacterUpdate{}
	c.mu.Unlock()

	for realmID, updates := range pending {
		groups := map[uint]*repo.Group{}
		groupsUpdates := map[uint][]events.GroupMemberStatsUpdate{}

		for charGUID, upd := range updates {
			groupID, err := c.repo.GroupIDByPlayer(ctx, realmID, charGUID)
			if err != nil {
				c.logger.Error().Err(err).Msg("can't get group id for character stats update")
				continue
			}

			if groupID == 0 {
				continue
			}

			group := groups[groupID]
			if group == nil {
				group, err = c.repo.GroupByID(ctx, realmID, groupID, true)
				if err != nil {
					c.logger.Error().Err(err).Msg("can't get group for character stats update")
					continue
				}
				if group == nil {
					continue
				}
				groups[groupID] = group
			}

			groupsUpdates[groupID] = append(groupsUpdates[groupID], events.GroupMemberStatsUpdate{
				MemberGUID: charGUID,
				Level:      upd.Lvl,
				Zone:       upd.Zone,
				CurHP:      upd.CurHP,
				MaxHP:      upd.MaxHP,
				PowerType:  upd.PowerType,
				CurPower:   upd.CurPower,
				MaxPower:   upd.MaxPower,
			})
		}

		for groupID, upds := range groupsUpdates {
			err := c.producer.MembersUpdated(&events.GroupEventGroupMembersUpdatedPayload{
				ServiceID: groupserver.ServiceID,
				RealmID:   realmID,
				GroupID:   groupID,
				Updates:   upds,
				Receivers: groups[groupID].OnlineMemberGUIDs(),
			})
			if err != nil {
				c.logger.Error().Err(err).Msg("can't publish group members updated event")
			}
		}
	}
}

func mergeCharacterUpdate(dst *events.CharacterUpdate, src *events.CharacterUpdate) {
	if src.Lvl != nil {
		dst.Lvl = src.Lvl
	}

	if src.Map != nil {
		dst.Map = src.Map
	}

	if src.Area != nil {
		dst.Area = src.Area
	}

	if src.Zone != nil {
		dst.Zone = src.Zone
	}

	if src.CurHP != nil {
		dst.CurHP = src.CurHP
	}

	if src.MaxHP != nil {
		dst.MaxHP = src.MaxHP
	}

	if src.PowerType != nil {
		dst.PowerType = src.PowerType
	}

	if src.CurPower != nil {
		dst.CurPower = src.CurPower
	}

	if src.MaxPower != nil {
		dst.MaxPower = src.MaxPower
	}
}
