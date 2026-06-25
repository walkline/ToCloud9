package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/groupserver"
	"github.com/walkline/ToCloud9/apps/groupserver/repo"
	"github.com/walkline/ToCloud9/gen/characters/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/groupstatetrace"
	"github.com/walkline/ToCloud9/shared/wow"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

type readyCheckKey struct {
	realmID uint32
	groupID uint
}

type pendingSubGroupSwapKey struct {
	realmID     uint32
	groupID     uint
	updaterGUID uint64
}

type pendingSubGroupSwap struct {
	memberGUID      uint64
	fromSubGroup    uint8
	toSubGroup      uint8
	expiresUnixNano int64
}

type memberStateTimestampKey struct {
	realmID    uint32
	memberGUID uint64
}

type memberPlacement struct {
	gatewayID     string
	worldserverID string
	online        bool
	mapID         uint32
	instanceID    uint32
	instanceKnown bool
	timestampMs   uint64
	updatedAt     time.Time
}

type memberLastState struct {
	snapshot  MemberStateSnapshot
	updatedAt time.Time
}

type memberStateTracker struct {
	mu                         sync.Mutex
	timestamps                 map[memberStateTimestampKey]uint64
	gatewayLifecycleEventTimes map[memberStateTimestampKey]uint64
	placements                 map[memberStateTimestampKey]memberPlacement
	states                     map[memberStateTimestampKey]memberLastState
}

func memberStateKey(defaultRealmID uint32, playerGUID uint64) memberStateTimestampKey {
	return memberStateTimestampKey{
		realmID:    guid.PlayerRealmIDOrDefault(defaultRealmID, playerGUID),
		memberGUID: guid.PlayerLowGUID(playerGUID),
	}
}

type offlineLeaderPromotionKey struct {
	realmID    uint32
	groupID    uint
	leaderGUID uint64
}

var (
	ErrAlreadyInGroup        = errors.New("player already in group")
	ErrNoPermissions         = errors.New("player has not enough permissions")
	ErrLFGGroupRestricted    = errors.New("lfg group does not allow this operation")
	ErrInvalidGroupOperation = errors.New("invalid group operation")
	ErrGroupFull             = errors.New("group is full")
	ErrGroupNotFound         = errors.New("group not found")
	ErrGroupMemberNotFound   = errors.New("group member not found")
	ErrMemberInDungeonOrRaid = errors.New("group member is in dungeon or raid")
	ErrInviteNotFound        = errors.New("invite not found")

	readyCheckMu        sync.Mutex
	readyCheckSequences = map[readyCheckKey]uint64{}

	pendingSubGroupSwapMu sync.Mutex
	pendingSubGroupSwaps  = map[pendingSubGroupSwapKey]pendingSubGroupSwap{}

	offlineLeaderPromotionDelay     = 2 * time.Minute
	offlineLeaderPromotionMu        sync.Mutex
	offlineLeaderPromotionSequence  uint64
	offlineLeaderPromotionSequences = map[offlineLeaderPromotionKey]uint64{}
)

type MessageType uint8

const (
	MessageTypeGroup       MessageType = 0x2
	MessageTypeGroupLeader MessageType = 0x33
	MessageTypeRaid        MessageType = 0x3
	MessageTypeRaidLeader  MessageType = 0x27
	MessageTypeRaidWarning MessageType = 0x28

	subGroupSwapPendingFlag uint8 = 0x80

	// Mirrors AzerothCore MAX_DUNGEON_DIFFICULTY and MAX_RAID_DIFFICULTY.
	maxDungeonDifficulty uint8 = 3
	maxRaidDifficulty    uint8 = 4

	memberPlacementFreshness = 15 * time.Second
)

func startReadyCheckTimeout(realmID uint32, groupID uint) uint64 {
	readyCheckMu.Lock()
	defer readyCheckMu.Unlock()

	key := readyCheckKey{realmID: realmID, groupID: groupID}
	readyCheckSequences[key]++
	return readyCheckSequences[key]
}

func consumeReadyCheckTimeout(realmID uint32, groupID uint, sequence uint64) bool {
	readyCheckMu.Lock()
	defer readyCheckMu.Unlock()

	key := readyCheckKey{realmID: realmID, groupID: groupID}
	if readyCheckSequences[key] != sequence {
		return false
	}

	delete(readyCheckSequences, key)
	return true
}

func clearReadyCheckTimeout(realmID uint32, groupID uint) {
	readyCheckMu.Lock()
	defer readyCheckMu.Unlock()

	delete(readyCheckSequences, readyCheckKey{realmID: realmID, groupID: groupID})
}

func consumePendingSubGroupSwap(realmID uint32, groupID uint, updaterGUID uint64, memberGUID uint64, fromSubGroup, toSubGroup uint8) (pendingSubGroupSwap, bool) {
	pendingSubGroupSwapMu.Lock()
	defer pendingSubGroupSwapMu.Unlock()

	key := pendingSubGroupSwapKey{realmID: realmID, groupID: groupID, updaterGUID: updaterGUID}
	pending, ok := pendingSubGroupSwaps[key]
	if !ok {
		return pendingSubGroupSwap{}, false
	}

	if time.Now().UnixNano() > pending.expiresUnixNano {
		delete(pendingSubGroupSwaps, key)
		return pendingSubGroupSwap{}, false
	}

	if pending.memberGUID == memberGUID || pending.fromSubGroup != toSubGroup || pending.toSubGroup != fromSubGroup {
		return pendingSubGroupSwap{}, false
	}

	delete(pendingSubGroupSwaps, key)
	return pending, true
}

func queuePendingSubGroupSwap(realmID uint32, groupID uint, updaterGUID uint64, memberGUID uint64, fromSubGroup, toSubGroup uint8) error {
	pendingSubGroupSwapMu.Lock()
	defer pendingSubGroupSwapMu.Unlock()

	now := time.Now()
	pruneExpiredPendingSubGroupSwapsLocked(now.UnixNano())

	key := pendingSubGroupSwapKey{realmID: realmID, groupID: groupID, updaterGUID: updaterGUID}
	if pending, ok := pendingSubGroupSwaps[key]; ok && now.UnixNano() <= pending.expiresUnixNano {
		return ErrGroupFull
	}

	pendingSubGroupSwaps[key] = pendingSubGroupSwap{
		memberGUID:      memberGUID,
		fromSubGroup:    fromSubGroup,
		toSubGroup:      toSubGroup,
		expiresUnixNano: now.Add(2 * time.Second).UnixNano(),
	}

	return nil
}

func pruneExpiredPendingSubGroupSwapsLocked(nowUnixNano int64) {
	for key, pending := range pendingSubGroupSwaps {
		if nowUnixNano > pending.expiresUnixNano {
			delete(pendingSubGroupSwaps, key)
		}
	}
}

func clearPendingSubGroupSwapsForMember(realmID uint32, groupID uint, memberGUID uint64) {
	pendingSubGroupSwapMu.Lock()
	defer pendingSubGroupSwapMu.Unlock()

	for key, pending := range pendingSubGroupSwaps {
		if key.realmID == realmID && key.groupID == groupID && (key.updaterGUID == memberGUID || pending.memberGUID == memberGUID) {
			delete(pendingSubGroupSwaps, key)
		}
	}
}

func clearPendingSubGroupSwapsForGroup(realmID uint32, groupID uint) {
	pendingSubGroupSwapMu.Lock()
	defer pendingSubGroupSwapMu.Unlock()

	for key := range pendingSubGroupSwaps {
		if key.realmID == realmID && key.groupID == groupID {
			delete(pendingSubGroupSwaps, key)
		}
	}
}

func newMemberStateTracker() *memberStateTracker {
	return &memberStateTracker{
		timestamps:                 map[memberStateTimestampKey]uint64{},
		gatewayLifecycleEventTimes: map[memberStateTimestampKey]uint64{},
		placements:                 map[memberStateTimestampKey]memberPlacement{},
		states:                     map[memberStateTimestampKey]memberLastState{},
	}
}

func (t *memberStateTracker) acceptTimestamp(realmID uint32, memberGUID uint64, timestampMs uint64, allowEqualBump bool) (uint64, bool) {
	if timestampMs == 0 {
		return timestampMs, true
	}

	key := memberStateKey(realmID, memberGUID)
	t.mu.Lock()
	defer t.mu.Unlock()

	if last, ok := t.timestamps[key]; ok {
		if timestampMs < last {
			return timestampMs, false
		}
		if timestampMs == last {
			if !allowEqualBump || last == ^uint64(0) {
				return timestampMs, false
			}
			timestampMs = last + 1
		}
	}

	t.timestamps[key] = timestampMs
	return timestampMs, true
}

func (t *memberStateTracker) hasTimestamp(realmID uint32, memberGUID uint64) bool {
	key := memberStateKey(realmID, memberGUID)
	t.mu.Lock()
	defer t.mu.Unlock()

	_, ok := t.timestamps[key]
	return ok
}

func (t *memberStateTracker) clearMember(realmID uint32, memberGUID uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := memberStateKey(realmID, memberGUID)
	// Placement is session-scoped, not group-scoped. Keep it across a leave or
	// disband so immediate regroup flows such as LFG can still route to the
	// owning worldserver before the next gateway state flush.
	delete(t.states, key)
}

func (t *memberStateTracker) clearGroup(realmID uint32, members []repo.GroupMember) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, member := range members {
		key := memberStateKey(realmID, member.MemberGUID)
		delete(t.states, key)
	}
}

func (t *memberStateTracker) recordMemberState(update memberStateUpdate, snapshot MemberStateSnapshot) {
	if snapshot.MemberGUID == 0 {
		return
	}

	key := memberStateKey(update.realmID, snapshot.MemberGUID)
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()

	t.recordMemberPlacementLocked(key, update, snapshot, now)

	if snapshot.Online {
		if isAuraOnlyMemberStateSnapshot(snapshot) {
			if last, ok := t.states[key]; ok && last.snapshot.Online {
				last.snapshot.AurasKnown = true
				last.snapshot.Auras = append([]MemberAuraState(nil), snapshot.Auras...)
				if snapshot.Dead != nil {
					last.snapshot.Dead = snapshot.Dead
				}
				if snapshot.Ghost != nil {
					last.snapshot.Ghost = snapshot.Ghost
				}
				if snapshot.TimestampMs != 0 {
					last.snapshot.TimestampMs = snapshot.TimestampMs
				}
				last.updatedAt = now
				t.states[key] = last
			}
			return
		}
		t.states[key] = memberLastState{snapshot: snapshot, updatedAt: now}
	} else {
		delete(t.states, key)
	}
}

func (t *memberStateTracker) recordMemberPlacement(update memberStateUpdate, snapshot MemberStateSnapshot) {
	if snapshot.MemberGUID == 0 {
		return
	}

	key := memberStateKey(update.realmID, snapshot.MemberGUID)
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()

	if snapshot.TimestampMs != 0 {
		if last, ok := t.timestamps[key]; ok && snapshot.TimestampMs < last {
			return
		}
	}
	t.recordMemberPlacementLocked(key, update, snapshot, now)
}

func (t *memberStateTracker) recordMemberPlacementLocked(key memberStateTimestampKey, update memberStateUpdate, snapshot MemberStateSnapshot, now time.Time) {
	hasPlacementEvidence := update.sourceGatewayID != "" || update.sourceWorldserverID != ""
	if !hasPlacementEvidence && snapshot.Online {
		return
	}

	placement, hadPlacement := t.placements[key]
	sourceChanged := hadPlacement && placement.worldserverID != "" && update.sourceWorldserverID != "" && placement.worldserverID != update.sourceWorldserverID
	gatewayChanged := hadPlacement && placement.gatewayID != "" && update.sourceGatewayID != "" && placement.gatewayID != update.sourceGatewayID
	mapChanged := hadPlacement && placement.mapID != snapshot.MapID
	if update.sourceGatewayID != "" {
		placement.gatewayID = update.sourceGatewayID
	}
	if update.sourceWorldserverID != "" {
		placement.worldserverID = update.sourceWorldserverID
	}
	placement.online = snapshot.Online
	if !snapshot.Online {
		placement.worldserverID = ""
	}
	placement.mapID = snapshot.MapID
	if snapshot.InstanceID != nil {
		placement.instanceID = *snapshot.InstanceID
		placement.instanceKnown = *snapshot.InstanceID != 0
	} else if !snapshot.Online || sourceChanged || gatewayChanged || mapChanged {
		placement.instanceID = 0
		placement.instanceKnown = false
	}
	if snapshot.TimestampMs != 0 {
		placement.timestampMs = snapshot.TimestampMs
	}
	placement.updatedAt = now
	t.placements[key] = placement
}

func (t *memberStateTracker) memberStateUpdatesForGroup(realmID uint32, members []repo.GroupMember, excludeGUID uint64) []events.GroupMemberStateUpdate {
	t.mu.Lock()
	defer t.mu.Unlock()

	updates := make([]events.GroupMemberStateUpdate, 0, len(members))
	for _, member := range members {
		if member.MemberGUID == 0 || guid.SamePlayer(realmID, member.MemberGUID, realmID, excludeGUID) || !member.IsOnline {
			continue
		}

		last, ok := t.states[memberStateKey(realmID, member.MemberGUID)]
		if !ok || !last.snapshot.Online {
			continue
		}

		update := memberStateSnapshotToUpdate(last.snapshot)
		update.MemberGUID = member.MemberGUID
		updates = append(updates, update)
	}

	sort.Slice(updates, func(i, j int) bool {
		return updates[i].MemberGUID < updates[j].MemberGUID
	})

	return updates
}

func (t *memberStateTracker) hasOnlineState(realmID uint32, memberGUID uint64) bool {
	if memberGUID == 0 {
		return false
	}

	key := memberStateKey(realmID, memberGUID)
	t.mu.Lock()
	defer t.mu.Unlock()

	last, ok := t.states[key]
	return ok && last.snapshot.Online
}

func (t *memberStateTracker) preserveKnownVitals(realmID uint32, snapshot MemberStateSnapshot) MemberStateSnapshot {
	if !snapshot.Online || snapshot.MemberGUID == 0 {
		return snapshot
	}

	key := memberStateKey(realmID, snapshot.MemberGUID)
	t.mu.Lock()
	last, ok := t.states[key]
	t.mu.Unlock()
	if !ok || !last.snapshot.Online {
		return snapshot
	}
	if isAuraOnlyMemberStateSnapshot(snapshot) {
		snapshot.Health = last.snapshot.Health
		snapshot.MaxHealth = last.snapshot.MaxHealth
		snapshot.PowerType = last.snapshot.PowerType
		snapshot.Power = last.snapshot.Power
		snapshot.MaxPower = last.snapshot.MaxPower
		if snapshot.Dead == nil && last.snapshot.Dead != nil {
			snapshot.Dead = last.snapshot.Dead
		}
		if snapshot.Ghost == nil && last.snapshot.Ghost != nil {
			snapshot.Ghost = last.snapshot.Ghost
		}
		return snapshot
	}

	if snapshot.MaxHealth == 0 && last.snapshot.MaxHealth > 0 {
		if snapshot.Health == 0 && snapshot.AurasKnown && !boolPtrValue(snapshot.Dead) {
			snapshot.Health = last.snapshot.Health
		}
		snapshot.MaxHealth = last.snapshot.MaxHealth
	}
	if snapshot.MaxPower == 0 && last.snapshot.MaxPower > 0 {
		if snapshot.Power == 0 && snapshot.AurasKnown {
			snapshot.Power = last.snapshot.Power
		}
		if snapshot.PowerType == 0 && last.snapshot.PowerType != 0 {
			snapshot.PowerType = last.snapshot.PowerType
		}
		snapshot.MaxPower = last.snapshot.MaxPower
	}
	if !snapshot.AurasKnown && last.snapshot.AurasKnown {
		snapshot.AurasKnown = true
		snapshot.Auras = append([]MemberAuraState(nil), last.snapshot.Auras...)
	}
	if snapshot.Dead == nil && last.snapshot.Dead != nil {
		snapshot.Dead = last.snapshot.Dead
	}
	if snapshot.Ghost == nil && last.snapshot.Ghost != nil {
		snapshot.Ghost = last.snapshot.Ghost
	}

	return snapshot
}

func isAuraOnlyMemberStateSnapshot(snapshot MemberStateSnapshot) bool {
	return snapshot.Online &&
		snapshot.AurasKnown &&
		!boolPtrValue(snapshot.Dead) &&
		!boolPtrValue(snapshot.Ghost) &&
		snapshot.Health == 0 &&
		snapshot.MaxHealth == 0 &&
		snapshot.Power == 0 &&
		snapshot.MaxPower == 0
}

func boolPtrValue(v *bool) bool {
	return v != nil && *v
}

func (t *memberStateTracker) recordGatewayLogin(payload events.GWEventCharacterLoggedInPayload) bool {
	if payload.CharGUID == 0 {
		return false
	}

	key := memberStateKey(payload.RealmID, payload.CharGUID)
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.acceptGatewayLifecycleEventLocked(key, payload.EventTimeUnixNano) {
		return false
	}

	placement := t.placements[key]
	if payload.GatewayID != "" {
		placement.gatewayID = payload.GatewayID
	}
	placement.worldserverID = ""
	placement.instanceID = 0
	placement.instanceKnown = false
	placement.online = true
	placement.mapID = payload.CharMap
	placement.updatedAt = time.Now()
	t.placements[key] = placement
	return true
}

func (t *memberStateTracker) recordGatewayLogout(payload events.GWEventCharacterLoggedOutPayload) bool {
	if payload.CharGUID == 0 {
		return false
	}

	key := memberStateKey(payload.RealmID, payload.CharGUID)
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.acceptGatewayLifecycleEventLocked(key, payload.EventTimeUnixNano) {
		return false
	}

	placement := t.placements[key]
	if payload.GatewayID != "" {
		placement.gatewayID = payload.GatewayID
	}
	placement.worldserverID = ""
	placement.instanceID = 0
	placement.instanceKnown = false
	placement.online = false
	placement.updatedAt = time.Now()
	t.placements[key] = placement
	return true
}

func (t *memberStateTracker) acceptGatewayLifecycleEventLocked(key memberStateTimestampKey, eventTimeUnixNano uint64) bool {
	if eventTimeUnixNano == 0 {
		return true
	}
	if t.gatewayLifecycleEventTimes == nil {
		t.gatewayLifecycleEventTimes = map[memberStateTimestampKey]uint64{}
	}
	if last := t.gatewayLifecycleEventTimes[key]; last > eventTimeUnixNano {
		return false
	}
	t.gatewayLifecycleEventTimes[key] = eventTimeUnixNano
	return true
}

func (t *memberStateTracker) onlineMembersForGateway(realmID uint32, gatewayID string, startedAt time.Time) []uint64 {
	if gatewayID == "" {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	members := make([]uint64, 0)
	for key, placement := range t.placements {
		if key.realmID == realmID &&
			placement.gatewayID == gatewayID &&
			placement.online &&
			(startedAt.IsZero() || placement.updatedAt.IsZero() || !placement.updatedAt.After(startedAt)) {
			members = append(members, key.memberGUID)
		}
	}
	sort.Slice(members, func(i, j int) bool {
		return members[i] < members[j]
	})

	return members
}

func (t *memberStateTracker) nextTimestampAfterLast(realmID uint32, memberGUID uint64, timestampMs uint64) uint64 {
	if timestampMs == 0 {
		return 0
	}

	key := memberStateKey(realmID, memberGUID)
	t.mu.Lock()
	defer t.mu.Unlock()

	if last, ok := t.timestamps[key]; ok && last != ^uint64(0) && timestampMs <= last {
		return last + 1
	}

	return timestampMs
}

func (t *memberStateTracker) placement(realmID uint32, memberGUID uint64) (memberPlacement, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	placement, ok := t.placements[memberStateKey(realmID, memberGUID)]
	return placement, ok
}

func (t *memberStateTracker) freshPlacement(realmID uint32, memberGUID uint64, now time.Time, maxAge time.Duration) (memberPlacement, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	placement, ok := t.placements[memberStateKey(realmID, memberGUID)]
	if !ok || placement.updatedAt.IsZero() || now.Sub(placement.updatedAt) > maxAge {
		return memberPlacement{}, false
	}

	return placement, true
}

func (t *memberStateTracker) memberHasOnlineEvidence(realmID uint32, memberGUID uint64) bool {
	if memberGUID == 0 {
		return false
	}

	key := memberStateKey(realmID, memberGUID)
	t.mu.Lock()
	defer t.mu.Unlock()

	if placement, ok := t.placements[key]; ok {
		return placement.online
	}

	last, ok := t.states[key]
	return ok && last.snapshot.Online
}

func queueOfflineLeaderPromotionTimer(realmID uint32, groupID uint, leaderGUID uint64) (uint64, bool) {
	offlineLeaderPromotionMu.Lock()
	defer offlineLeaderPromotionMu.Unlock()

	key := offlineLeaderPromotionKey{realmID: realmID, groupID: groupID, leaderGUID: leaderGUID}
	if sequence, ok := offlineLeaderPromotionSequences[key]; ok {
		return sequence, false
	}

	offlineLeaderPromotionSequence++
	offlineLeaderPromotionSequences[key] = offlineLeaderPromotionSequence
	return offlineLeaderPromotionSequence, true
}

func consumeOfflineLeaderPromotionTimer(realmID uint32, groupID uint, leaderGUID uint64, sequence uint64) bool {
	offlineLeaderPromotionMu.Lock()
	defer offlineLeaderPromotionMu.Unlock()

	key := offlineLeaderPromotionKey{realmID: realmID, groupID: groupID, leaderGUID: leaderGUID}
	if offlineLeaderPromotionSequences[key] != sequence {
		return false
	}

	delete(offlineLeaderPromotionSequences, key)
	return true
}

func clearOfflineLeaderPromotionTimer(realmID uint32, groupID uint, leaderGUID uint64) {
	offlineLeaderPromotionMu.Lock()
	defer offlineLeaderPromotionMu.Unlock()

	delete(offlineLeaderPromotionSequences, offlineLeaderPromotionKey{realmID: realmID, groupID: groupID, leaderGUID: leaderGUID})
}

func clearGroupOfflineLeaderPromotionTimers(realmID uint32, groupID uint) {
	offlineLeaderPromotionMu.Lock()
	defer offlineLeaderPromotionMu.Unlock()

	for key := range offlineLeaderPromotionSequences {
		if key.realmID == realmID && key.groupID == groupID {
			delete(offlineLeaderPromotionSequences, key)
		}
	}
}

type GroupsService interface {
	GroupByID(ctx context.Context, realmID uint32, groupID uint) (*repo.Group, error)
	GroupByMemberGUID(ctx context.Context, realmID uint32, memberGUID uint64) (*repo.Group, error)
	GroupIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint, error)
	GroupRealmIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint32, uint, error)
	MemberPlacements(ctx context.Context, realmID uint32, memberGUIDs []uint64) ([]MemberPlacementSnapshot, error)

	Invite(ctx context.Context, realmID uint32, inviter, invited uint64, inviterName, invitedName string) error
	Uninvite(ctx context.Context, realmID uint32, initiator, target uint64, reason string) error
	Leave(ctx context.Context, realmID uint32, player uint64) error

	ChangeLeader(ctx context.Context, realmID uint32, player, newLeader uint64) error
	ConvertToRaid(ctx context.Context, realmID uint32, player uint64) error

	AcceptInvite(ctx context.Context, realmID uint32, player uint64) error
	DeclineInvite(ctx context.Context, realmID uint32, player uint64) error

	SendMessage(ctx context.Context, realmID uint32, senderGUID uint64, message string, lang uint32, messageType MessageType, senderChatTag uint8) error

	SetTargetIcon(ctx context.Context, realmID uint32, updaterGUID uint64, iconID uint8, targetGUID uint64) error
	SetLootMethod(ctx context.Context, realmID uint32, updaterGUID uint64, method uint8, lootMaster uint64, lootThreshold uint8) error

	SetDungeonDifficulty(ctx context.Context, realmID uint32, updaterGUID uint64, difficulty uint8) error
	SetRaidDifficulty(ctx context.Context, realmID uint32, updaterGUID uint64, difficulty uint8) error

	StartReadyCheck(ctx context.Context, realmID uint32, leaderGUID uint64, durationMs uint32) error
	SetReadyCheckMemberState(ctx context.Context, realmID uint32, memberGUID uint64, state uint8) error
	FinishReadyCheck(ctx context.Context, realmID uint32, playerGUID uint64) error
	ChangeMemberSubGroup(ctx context.Context, realmID uint32, updaterGUID, memberGUID uint64, subGroup uint8) error
	SetMemberFlags(ctx context.Context, realmID uint32, updaterGUID, memberGUID uint64, flags, roles uint8) error
	RegisterAcceptedLfgGroup(ctx context.Context, realmID, proposalID, dungeonEntry, leaderRealmID uint32, leaderGUID uint64, crossRealm bool, members []AcceptedLfgGroupMember) (uint, error)
	RegisterMaterializedLfgGroup(ctx context.Context, realmID uint32, groupID uint, leaderGUID uint64, groupType, difficulty, raidDifficulty uint8, members []MaterializedLfgGroupMember) error
	UpdateMemberState(ctx context.Context, realmID uint32, memberGUID uint64, online bool, level, class uint8, zoneID, mapID uint32, health, maxHealth uint32, powerType uint8, power, maxPower uint32, instanceID *uint32) error
	BulkUpdateMemberStates(ctx context.Context, realmID uint32, sourceGatewayID, sourceWorldserverID string, snapshots []MemberStateSnapshot) error
	ResetInstance(ctx context.Context, realmID uint32, playerGUID uint64, mapID uint32, difficulty uint8) error
	SetInstanceBindExtension(ctx context.Context, realmID uint32, playerGUID uint64, mapID uint32, difficulty uint8, extended bool) error

	// GWCharacterLoggedInHandler updates cache with player logged in.
	events.GWCharacterLoggedInHandler
	// GWGatewayStartedHandler expires state owned by a restarted gateway.
	events.GWGatewayStartedHandler
	// GWCharacterLoggedOutHandler updates cache with player logged out.
	events.GWCharacterLoggedOutHandler
}

type MemberStateSnapshot struct {
	MemberGUID  uint64
	Online      bool
	Level       uint8
	Class       uint8
	ZoneID      uint32
	MapID       uint32
	Health      uint32
	MaxHealth   uint32
	PowerType   uint8
	Power       uint32
	MaxPower    uint32
	InstanceID  *uint32
	AurasKnown  bool
	Auras       []MemberAuraState
	TimestampMs uint64
	Dead        *bool
	Ghost       *bool
}

type MaterializedLfgGroupMember struct {
	RealmID    uint32
	PlayerGUID uint64
	Name       string
	Online     bool
	Flags      uint8
	Roles      uint8
	SubGroup   uint8
}

type AcceptedLfgGroupMember struct {
	RealmID            uint32
	PlayerGUID         uint64
	SelectedRoles      uint8
	AssignedRole       uint8
	QueueLeaderRealmID uint32
	QueueLeaderGUID    uint64
}

type MemberAuraState struct {
	Slot    uint8
	SpellID uint32
	Flags   uint8
}

type MemberPlacementSnapshot struct {
	MemberGUID    uint64
	Online        bool
	Fresh         bool
	GatewayID     string
	WorldserverID string
	MapID         uint32
	InstanceID    uint32
	InstanceKnown bool
	TimestampMs   uint64
	UpdatedAtMs   uint64
}

type memberStateUpdate struct {
	realmID             uint32
	sourceGatewayID     string
	sourceWorldserverID string
	snapshot            MemberStateSnapshot
}

type memberStateEvent struct {
	realmID             uint32
	groupID             uint
	sourceGatewayID     string
	sourceWorldserverID string
	receivers           []uint64
	state               events.GroupMemberStateUpdate
}

type memberStateBatchKey struct {
	realmID             uint32
	groupID             uint
	sourceGatewayID     string
	sourceWorldserverID string
	receiversKey        string
}

type memberStateBatch struct {
	realmID             uint32
	groupID             uint
	sourceGatewayID     string
	sourceWorldserverID string
	receivers           []uint64
	states              []events.GroupMemberStateUpdate
}

func subgroupMemberCount(group *repo.Group, subGroup uint8, exceptGUID uint64) int {
	count := 0
	groupRealmID := groupHomeRealmID(0, group)

	for _, member := range group.Members {
		if guid.SamePlayer(groupRealmID, member.MemberGUID, groupRealmID, exceptGUID) {
			continue
		}

		if member.SubGroup == subGroup {
			count++
		}
	}

	return count
}

func nextLeaderAfterMemberLeaves(group *repo.Group, leavingGUID uint64) uint64 {
	var fallback uint64
	groupRealmID := groupHomeRealmID(0, group)
	for _, member := range group.Members {
		if guid.SamePlayer(groupRealmID, member.MemberGUID, groupRealmID, leavingGUID) {
			continue
		}

		if fallback == 0 {
			fallback = member.MemberGUID
		}

		if member.IsOnline {
			return member.MemberGUID
		}
	}

	return fallback
}

func nextLeaderAfterOfflineLeader(group *repo.Group, leaderGUID uint64) uint64 {
	groupRealmID := groupHomeRealmID(0, group)
	if group.IsRaid() {
		for _, member := range group.Members {
			if !guid.SamePlayer(groupRealmID, member.MemberGUID, groupRealmID, leaderGUID) && member.IsOnline && member.IsAssistant() {
				return member.MemberGUID
			}
		}
	}

	for _, member := range group.Members {
		if !guid.SamePlayer(groupRealmID, member.MemberGUID, groupRealmID, leaderGUID) && member.IsOnline {
			return member.MemberGUID
		}
	}

	return 0
}

func NewGroupsService(r repo.GroupsRepo, charClient pb.CharactersServiceClient, ep events.GroupServiceProducer) GroupsService {
	return &groupServiceImpl{
		r:                  r,
		ep:                 ep,
		charClient:         charClient,
		memberStateTracker: newMemberStateTracker(),
	}
}

type groupServiceImpl struct {
	r  repo.GroupsRepo
	ep events.GroupServiceProducer

	charClient pb.CharactersServiceClient

	memberStateTracker *memberStateTracker
}

type groupMemberOnlineStatusUpdater interface {
	SetMemberOnlineStatus(ctx context.Context, realmID uint32, memberGUID uint64, online bool) (*repo.Group, error)
}

type groupRealmIDByPlayerResolver interface {
	GroupRealmIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint32, uint, error)
}

type materializedLfgGroupRegistrar interface {
	RegisterMaterializedLfgGroup(ctx context.Context, realmID uint32, group *repo.Group) error
}

type acceptedLfgGroupRegistrar interface {
	RegisterAcceptedLfgGroup(ctx context.Context, realmID uint32, group *repo.Group) error
}

func (g *groupServiceImpl) stateTracker() *memberStateTracker {
	if g.memberStateTracker == nil {
		g.memberStateTracker = newMemberStateTracker()
	}
	return g.memberStateTracker
}

func (g groupServiceImpl) GroupIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint, error) {
	return g.r.GroupIDByPlayer(ctx, realmID, player)
}

func (g groupServiceImpl) GroupRealmIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint32, uint, error) {
	return g.groupRealmIDByPlayer(ctx, realmID, player)
}

func (g groupServiceImpl) groupRealmIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint32, uint, error) {
	if resolver, ok := g.r.(groupRealmIDByPlayerResolver); ok {
		return resolver.GroupRealmIDByPlayer(ctx, realmID, player)
	}

	groupID, err := g.r.GroupIDByPlayer(ctx, realmID, player)
	return realmID, groupID, err
}

func groupHomeRealmID(fallbackRealmID uint32, group *repo.Group) uint32 {
	if group != nil && group.RealmID != 0 {
		return group.RealmID
	}
	return fallbackRealmID
}

func groupPlayerGUID(groupRealmID, requestRealmID uint32, playerGUID uint64) uint64 {
	return guid.PlayerGUIDForRealm(groupRealmID, guid.PlayerRealmIDOrDefault(requestRealmID, playerGUID), playerGUID)
}

func (g groupServiceImpl) shortOnlineCharactersForGroup(ctx context.Context, groupRealmID uint32, memberGUIDs []uint64) ([]*pb.ShortCharactersDataByGUIDsResponse_ShortCharData, error) {
	if len(memberGUIDs) == 0 {
		return nil, nil
	}

	guidsByRealm := make(map[uint32][]uint64)
	for _, memberGUID := range memberGUIDs {
		if memberGUID == 0 {
			continue
		}

		memberRealmID := guid.PlayerRealmIDOrDefault(groupRealmID, memberGUID)
		guidsByRealm[memberRealmID] = append(guidsByRealm[memberRealmID], guid.PlayerLowGUID(memberGUID))
	}

	characters := make([]*pb.ShortCharactersDataByGUIDsResponse_ShortCharData, 0, len(memberGUIDs))
	for memberRealmID, realmGUIDs := range guidsByRealm {
		response, err := g.charClient.ShortOnlineCharactersDataByGUIDs(ctx, &pb.ShortCharactersDataByGUIDsRequest{
			Api:     groupserver.Ver,
			RealmID: memberRealmID,
			GUIDs:   realmGUIDs,
		})
		if err != nil {
			return nil, err
		}
		if response != nil {
			characters = append(characters, response.Characters...)
		}
	}

	return characters, nil
}

func (g groupServiceImpl) GroupByID(ctx context.Context, realmID uint32, groupID uint) (*repo.Group, error) {
	if groupID == 0 {
		return nil, nil
	}
	return g.r.GroupByID(ctx, realmID, groupID, true)
}

func (g groupServiceImpl) GroupByMemberGUID(ctx context.Context, realmID uint32, memberGUID uint64) (*repo.Group, error) {
	groupRealmID, groupID, err := g.groupRealmIDByPlayer(ctx, realmID, memberGUID)
	if err != nil {
		return nil, err
	}

	return g.GroupByID(ctx, groupRealmID, groupID)
}

func (g groupServiceImpl) MemberPlacements(_ context.Context, realmID uint32, memberGUIDs []uint64) ([]MemberPlacementSnapshot, error) {
	now := time.Now()
	tracker := g.stateTracker()
	placements := make([]MemberPlacementSnapshot, 0, len(memberGUIDs))
	for _, memberGUID := range memberGUIDs {
		snapshot := MemberPlacementSnapshot{
			MemberGUID: memberGUID,
		}
		placement, fresh := tracker.freshPlacement(realmID, memberGUID, now, memberPlacementFreshness)
		if fresh {
			snapshot.Online = placement.online
			snapshot.Fresh = true
			snapshot.GatewayID = placement.gatewayID
			snapshot.WorldserverID = placement.worldserverID
			snapshot.MapID = placement.mapID
			snapshot.InstanceID = placement.instanceID
			snapshot.InstanceKnown = placement.instanceKnown
			snapshot.TimestampMs = placement.timestampMs
			snapshot.UpdatedAtMs = uint64(placement.updatedAt.UnixMilli())
		}
		placements = append(placements, snapshot)
	}
	return placements, nil
}

func (g groupServiceImpl) Invite(ctx context.Context, realmID uint32, inviter, invited uint64, inviterName, invitedName string) error {
	inviter = guid.NormalizePlayerGUIDForRealm(realmID, inviter)
	invitedRealmID := guid.PlayerRealmIDOrDefault(realmID, invited)
	invited = guid.PlayerGUIDForRealm(realmID, invitedRealmID, invited)

	_, groupID, err := g.groupRealmIDByPlayer(ctx, realmID, invited)
	if err != nil {
		return err
	}

	if groupID != 0 {
		return ErrAlreadyInGroup
	}

	inviterGroupRealmID, inviterGroupID, err := g.groupRealmIDByPlayer(ctx, realmID, inviter)
	if err != nil {
		return err
	}

	if inviterGroupID == 0 {
		if err = g.r.AddInvite(ctx, invitedRealmID, repo.GroupInvite{
			Inviter:        inviter,
			InviterRealmID: realmID,
			InviterName:    inviterName,
			Invitee:        invited,
			InviteeRealmID: invitedRealmID,
			InviteeName:    invitedName,
			GroupID:        0,
			GroupRealmID:   realmID,
		}); err != nil {
			return err
		}

		err = g.ep.InviteCreated(&events.GroupEventInviteCreatedPayload{
			ServiceID:   groupserver.ServiceID,
			RealmID:     realmID,
			GroupID:     0,
			InviterGUID: inviter,
			InviterName: inviterName,
			InviteeGUID: invited,
			InviteeName: invitedName,
		})

		if err != nil {
			log.Error().Err(err).Msg("can't create invite created event")
		}

		return nil
	}

	group, err := g.r.GroupByID(ctx, inviterGroupRealmID, inviterGroupID, true)
	if err != nil {
		return err
	}
	groupRealmID := groupHomeRealmID(inviterGroupRealmID, group)
	inviter = groupPlayerGUID(groupRealmID, realmID, inviter)

	member := group.MemberByGUID(inviter)
	if member == nil {
		return fmt.Errorf("can't find player %d in the guild %d", inviter, inviterGroupID)
	}

	if !guid.SamePlayer(groupRealmID, group.LeaderGUID, groupRealmID, inviter) && !member.IsAssistant() {
		return ErrNoPermissions
	}

	if group.IsFull() {
		return ErrGroupFull
	}

	invited = groupPlayerGUID(groupRealmID, invitedRealmID, invited)
	if err = g.r.AddInvite(ctx, invitedRealmID, repo.GroupInvite{
		Inviter:        inviter,
		InviterRealmID: realmID,
		InviterName:    inviterName,
		Invitee:        invited,
		InviteeRealmID: invitedRealmID,
		InviteeName:    invitedName,
		GroupID:        inviterGroupID,
		GroupRealmID:   groupRealmID,
	}); err != nil {
		return err
	}

	err = g.ep.InviteCreated(&events.GroupEventInviteCreatedPayload{
		ServiceID:   groupserver.ServiceID,
		RealmID:     groupRealmID,
		GroupID:     inviterGroupID,
		InviterGUID: inviter,
		InviterName: inviterName,
		InviteeGUID: invited,
		InviteeName: invitedName,
	})

	if err != nil {
		log.Error().Err(err).Msg("can't create invite created event")
	}

	return nil
}

func (g groupServiceImpl) AcceptInvite(ctx context.Context, realmID uint32, player uint64) error {
	invite, err := g.r.GetInviteByInvitedPlayer(ctx, realmID, player)
	if err != nil {
		return err
	}

	if invite == nil {
		return ErrInviteNotFound
	}

	if invite.GroupID == 0 {
		groupRealmID := invite.GroupRealmID
		if groupRealmID == 0 {
			groupRealmID = realmID
		}
		if err = g.createGroup(ctx, groupRealmID, invite); err != nil {
			return err
		}
		g.removeInviteAfterAccept(ctx, realmID, player)
		return nil
	}

	groupRealmID := invite.GroupRealmID
	if groupRealmID == 0 {
		groupRealmID = realmID
	}
	group, err := g.r.GroupByID(ctx, groupRealmID, invite.GroupID, true)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}

	if err = g.addMember(ctx, groupRealmID, group, invite); err != nil {
		return err
	}
	g.removeInviteAfterAccept(ctx, realmID, player)
	return nil
}

func (g groupServiceImpl) DeclineInvite(ctx context.Context, realmID uint32, player uint64) error {
	invite, err := g.r.GetInviteByInvitedPlayer(ctx, realmID, player)
	if err != nil {
		return err
	}

	if invite == nil {
		return ErrInviteNotFound
	}

	if err = g.r.RemoveInvite(ctx, realmID, player); err != nil {
		return err
	}

	groupRealmID := invite.GroupRealmID
	if groupRealmID == 0 {
		groupRealmID = realmID
	}
	err = g.ep.InviteDeclined(&events.GroupEventInviteDeclinedPayload{
		ServiceID:   groupserver.ServiceID,
		RealmID:     groupRealmID,
		InviterGUID: invite.Inviter,
		InviteeGUID: invite.Invitee,
		InviteeName: invite.InviteeName,
	})
	if err != nil {
		log.Error().Err(err).Msg("can't create invite declined event")
	}

	return nil
}

func (g groupServiceImpl) removeInviteAfterAccept(ctx context.Context, realmID uint32, player uint64) {
	if err := g.r.RemoveInvite(ctx, realmID, player); err != nil {
		log.Error().
			Err(err).
			Uint32("realmID", realmID).
			Uint64("player", player).
			Msg("can't remove accepted group invite")
	}
}

func (g *groupServiceImpl) Uninvite(ctx context.Context, realmID uint32, initiator, target uint64, reason string) error {
	if guid.SamePlayer(realmID, initiator, realmID, target) {
		return ErrNoPermissions
	}

	group, err := g.GroupByMemberGUID(ctx, realmID, initiator)
	if err != nil {
		return fmt.Errorf("can't get group, err: %w", err)
	}
	if group == nil {
		return ErrGroupNotFound
	}
	groupRealmID := groupHomeRealmID(realmID, group)
	initiator = groupPlayerGUID(groupRealmID, realmID, initiator)
	target = groupPlayerGUID(groupRealmID, realmID, target)

	targetMember := group.MemberByGUID(target)
	if targetMember == nil {
		return ErrGroupNotFound
	}

	if !guid.SamePlayer(groupRealmID, group.LeaderGUID, groupRealmID, initiator) {
		return ErrNoPermissions
	}

	membersCount := len(group.Members)

	if membersCount <= 2 {
		if err = g.disband(ctx, groupRealmID, group); err != nil {
			return fmt.Errorf("can't disband group, err: %w", err)
		}
	} else {
		eventToSend := events.GroupEventGroupMemberLeftPayload{
			ServiceID:     groupserver.ServiceID,
			RealmID:       groupRealmID,
			GroupID:       group.ID,
			MemberGUID:    targetMember.MemberGUID,
			MemberName:    targetMember.MemberName,
			NewLeaderID:   group.LeaderGUID,
			OnlineMembers: group.OnlineMemberGUIDs(),
		}
		if err = g.r.RemoveMember(ctx, groupRealmID, target); err != nil {
			return fmt.Errorf("can't remove member, err: %w", err)
		}
		clearPendingSubGroupSwapsForMember(groupRealmID, group.ID, target)
		g.stateTracker().clearMember(realmID, target)

		err = g.ep.GroupMemberLeft(&eventToSend)
		if err != nil {
			log.Error().Err(err).Msg("can't create GroupMemberLeft event")
		}
	}

	return nil
}

func (g *groupServiceImpl) Leave(ctx context.Context, realmID uint32, player uint64) error {
	group, err := g.GroupByMemberGUID(ctx, realmID, player)
	if err != nil {
		return fmt.Errorf("can't get group, err: %w", err)
	}
	if group == nil {
		return ErrGroupNotFound
	}
	groupRealmID := groupHomeRealmID(realmID, group)
	player = groupPlayerGUID(groupRealmID, realmID, player)

	member := group.MemberByGUID(player)
	if member == nil {
		return ErrGroupNotFound
	}

	if len(group.Members) <= 2 {
		return g.disband(ctx, groupRealmID, group)
	}

	if guid.SamePlayer(groupRealmID, player, groupRealmID, group.LeaderGUID) {
		newLeader := nextLeaderAfterMemberLeaves(group, player)
		if err = g.changeLeader(ctx, groupRealmID, group, newLeader, false); err != nil {
			return fmt.Errorf("can't change group leader, err: %w", err)
		}
	}

	eventToSend := events.GroupEventGroupMemberLeftPayload{
		ServiceID:     groupserver.ServiceID,
		RealmID:       groupRealmID,
		GroupID:       group.ID,
		MemberGUID:    member.MemberGUID,
		MemberName:    member.MemberName,
		NewLeaderID:   group.LeaderGUID,
		OnlineMembers: group.OnlineMemberGUIDs(),
	}

	if err = g.r.RemoveMember(ctx, groupRealmID, player); err != nil {
		return fmt.Errorf("can't remove group member, err: %w", err)
	}
	clearPendingSubGroupSwapsForMember(groupRealmID, group.ID, player)
	g.stateTracker().clearMember(realmID, player)

	err = g.ep.GroupMemberLeft(&eventToSend)
	if err != nil {
		log.Error().Err(err).Msg("can't create GroupMemberLeft event")
	}

	return nil
}

func (g groupServiceImpl) ChangeLeader(ctx context.Context, realmID uint32, player, newLeader uint64) error {
	group, err := g.getGroupWithLeader(ctx, realmID, player)
	if err != nil {
		return err
	}
	groupRealmID := groupHomeRealmID(realmID, group)
	newLeader = groupPlayerGUID(groupRealmID, realmID, newLeader)

	newLeaderMember := group.MemberByGUID(newLeader)
	if newLeaderMember == nil {
		return ErrGroupNotFound
	}

	return g.changeLeader(ctx, groupRealmID, group, newLeader, true)
}

func (g groupServiceImpl) ConvertToRaid(ctx context.Context, realmID uint32, player uint64) error {
	group, err := g.getGroupWithLeader(ctx, realmID, player)
	if err != nil {
		return err
	}
	groupRealmID := groupHomeRealmID(realmID, group)
	if group.IsLFG() {
		return ErrLFGGroupRestricted
	}
	if len(group.Members) < 2 {
		return ErrInvalidGroupOperation
	}

	group.GroupType |= repo.GroupTypeFlagsRaid
	if err := g.r.Update(ctx, groupRealmID, group); err != nil {
		return fmt.Errorf("can't update group win a new leader, err: %w", err)
	}
	err = g.ep.ConvertedToRaid(&events.GroupEventGroupConvertedToRaidPayload{
		ServiceID:     groupserver.ServiceID,
		RealmID:       groupRealmID,
		GroupID:       group.ID,
		Leader:        group.LeaderGUID,
		OnlineMembers: group.OnlineMemberGUIDs(),
	})
	if err != nil {
		log.Error().Err(err).Msg("can't create ConvertedToRaid event")
	}

	return nil
}

func (g groupServiceImpl) SendMessage(ctx context.Context, realmID uint32, senderGUID uint64, message string, lang uint32, messageType MessageType, senderChatTag uint8) error {
	group, err := g.GroupByMemberGUID(ctx, realmID, senderGUID)
	if err != nil {
		return fmt.Errorf("can't get group, err: %w", err)
	}

	if group == nil {
		return ErrGroupNotFound
	}
	groupRealmID := groupHomeRealmID(realmID, group)
	senderGUID = groupPlayerGUID(groupRealmID, realmID, senderGUID)

	member := group.MemberByGUID(senderGUID)
	if member == nil {
		return ErrGroupMemberNotFound
	}

	requiresLeader := false
	requiresLeaderOrAssistant := false
	switch messageType {
	case MessageTypeGroup, MessageTypeRaid:
	case MessageTypeGroupLeader, MessageTypeRaidLeader:
		requiresLeader = true
	case MessageTypeRaidWarning:
		requiresLeaderOrAssistant = true
	default:
		return fmt.Errorf("message with type %d unsupported", messageType)
	}

	if requiresLeader && !guid.SamePlayer(groupRealmID, group.LeaderGUID, groupRealmID, senderGUID) {
		return ErrNoPermissions
	}

	if requiresLeaderOrAssistant && !guid.SamePlayer(groupRealmID, group.LeaderGUID, groupRealmID, senderGUID) && !member.IsAssistant() {
		return ErrNoPermissions
	}

	err = g.ep.SendChatMessage(&events.GroupEventNewMessagePayload{
		ServiceID:     groupserver.ServiceID,
		RealmID:       groupRealmID,
		GroupID:       group.ID,
		SenderGUID:    senderGUID,
		SenderName:    member.MemberName,
		SenderChatTag: senderChatTag,
		Language:      lang,
		Msg:           message,
		MessageType:   uint8(messageType),
		Receivers:     group.OnlineMemberGUIDs(),
	})
	if err != nil {
		log.Error().Err(err).Msg("can't create SendChatMessage event")
	}

	return nil
}

func (g groupServiceImpl) SetTargetIcon(ctx context.Context, realmID uint32, updaterGUID uint64, iconID uint8, targetGUID uint64) error {
	if repo.MaxTargetIcons <= iconID {
		return fmt.Errorf("iconID (%d) is invalid", iconID)
	}

	group, err := g.GroupByMemberGUID(ctx, realmID, updaterGUID)
	if err != nil {
		return fmt.Errorf("can't get group, err: %w", err)
	}

	if group == nil {
		return ErrGroupNotFound
	}
	groupRealmID := groupHomeRealmID(realmID, group)
	updaterGUID = groupPlayerGUID(groupRealmID, realmID, updaterGUID)
	targetGUID = groupPlayerGUID(groupRealmID, realmID, targetGUID)

	groupMember := group.MemberByGUID(updaterGUID)
	if groupMember == nil {
		return ErrGroupMemberNotFound
	}
	if group.IsRaid() && !guid.SamePlayer(groupRealmID, group.LeaderGUID, groupRealmID, updaterGUID) && !groupMember.IsAssistant() {
		return ErrNoPermissions
	}

	if targetGUID != 0 {
		for i, target := range group.TargetIcons {
			if guid.SamePlayer(groupRealmID, target, groupRealmID, targetGUID) {
				group.TargetIcons[i] = 0

				err = g.ep.TargetIconUpdated(&events.GroupEventNewTargetIconPayload{
					ServiceID: groupserver.ServiceID,
					RealmID:   groupRealmID,
					GroupID:   group.ID,
					Updater:   0,
					Target:    0,
					IconID:    uint8(i),
					Receivers: group.OnlineMemberGUIDs(),
				})
				if err != nil {
					log.Error().Err(err).Msg("can't create TargetIconUpdated clear event")
				}

				break
			}
		}
	}

	group.TargetIcons[iconID] = targetGUID

	if err = g.r.Update(ctx, groupRealmID, group); err != nil {
		return fmt.Errorf("can't update icon for the group (%d), err: %w", group.ID, err)
	}

	err = g.ep.TargetIconUpdated(&events.GroupEventNewTargetIconPayload{
		ServiceID: groupserver.ServiceID,
		RealmID:   groupRealmID,
		GroupID:   group.ID,
		Updater:   updaterGUID,
		Target:    targetGUID,
		IconID:    iconID,
		Receivers: group.OnlineMemberGUIDs(),
	})
	if err != nil {
		log.Error().Err(err).Msg("can't create TargetIconUpdated event")
	}

	return nil
}

func (g groupServiceImpl) SetLootMethod(ctx context.Context, realmID uint32, updaterGUID uint64, method uint8, lootMaster uint64, lootThreshold uint8) error {
	group, err := g.getGroupWithLeader(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}
	groupRealmID := groupHomeRealmID(realmID, group)
	if group.IsLFG() {
		return ErrLFGGroupRestricted
	}
	lootMaster = groupPlayerGUID(groupRealmID, realmID, lootMaster)
	if method > uint8(repo.LootTypeNeedBeforeGreed) ||
		lootThreshold < uint8(repo.ItemQualityUncommon) ||
		lootThreshold > uint8(repo.ItemQualityArtifact) {
		return ErrInvalidGroupOperation
	}
	if method == uint8(repo.LootTypeMasterLoot) && group.MemberByGUID(lootMaster) == nil {
		return ErrInvalidGroupOperation
	}

	group.LootMethod = method
	group.LootThreshold = lootThreshold
	group.LooterGUID = lootMaster

	if err = g.r.Update(ctx, groupRealmID, group); err != nil {
		return err
	}

	err = g.ep.LootTypeChanged(&events.GroupEventGroupLootTypeChangedPayload{
		ServiceID:          groupserver.ServiceID,
		RealmID:            groupRealmID,
		GroupID:            group.ID,
		NewLootType:        group.LootMethod,
		NewLooterGUID:      group.LooterGUID,
		NewLooterThreshold: group.LootThreshold,
		OnlineMembers:      group.OnlineMemberGUIDs(),
	})
	if err != nil {
		log.Error().Err(err).Msg("can't send loot changed event")
	}

	return nil
}

func (g groupServiceImpl) SetDungeonDifficulty(ctx context.Context, realmID uint32, updaterGUID uint64, difficulty uint8) error {
	if difficulty >= maxDungeonDifficulty {
		return ErrInvalidGroupOperation
	}

	group, err := g.getGroupWithLeader(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}
	groupRealmID := groupHomeRealmID(realmID, group)

	characters, err := g.shortOnlineCharactersForGroup(ctx, groupRealmID, group.OnlineMemberGUIDs())
	if err != nil {
		return fmt.Errorf("failed to get characters, err: %w", err)
	}

	for _, char := range characters {
		if MapID(int(char.CharMap)).IsDungeon() {
			return ErrMemberInDungeonOrRaid
		}
	}

	group.Difficulty = difficulty

	if err = g.r.Update(ctx, groupRealmID, group); err != nil {
		return err
	}

	err = g.ep.GroupDifficultyChanged(&events.GroupEventGroupDifficultyChangedPayload{
		ServiceID:         groupserver.ServiceID,
		RealmID:           groupRealmID,
		GroupID:           group.ID,
		Updater:           updaterGUID,
		DungeonDifficulty: &difficulty,
		RaidDifficulty:    nil,
		Receivers:         group.OnlineMemberGUIDs(),
	})
	if err != nil {
		log.Error().Err(err).Msg("can't send difficulty changed event")
	}

	return nil
}

func (g groupServiceImpl) SetRaidDifficulty(ctx context.Context, realmID uint32, updaterGUID uint64, difficulty uint8) error {
	if difficulty >= maxRaidDifficulty {
		return ErrInvalidGroupOperation
	}

	group, err := g.getGroupWithLeader(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}
	groupRealmID := groupHomeRealmID(realmID, group)

	characters, err := g.shortOnlineCharactersForGroup(ctx, groupRealmID, group.OnlineMemberGUIDs())
	if err != nil {
		return fmt.Errorf("failed to get characters, err: %w", err)
	}

	for _, char := range characters {
		if MapID(int(char.CharMap)).IsRaid() {
			return ErrMemberInDungeonOrRaid
		}
	}

	group.RaidDifficulty = difficulty

	if err = g.r.Update(ctx, groupRealmID, group); err != nil {
		return err
	}

	err = g.ep.GroupDifficultyChanged(&events.GroupEventGroupDifficultyChangedPayload{
		ServiceID:         groupserver.ServiceID,
		RealmID:           groupRealmID,
		GroupID:           group.ID,
		Updater:           updaterGUID,
		DungeonDifficulty: nil,
		RaidDifficulty:    &difficulty,
		Receivers:         group.OnlineMemberGUIDs(),
	})
	if err != nil {
		log.Error().Err(err).Msg("can't send difficulty changed event")
	}

	return nil
}

func (g *groupServiceImpl) HandleCharacterLoggedIn(payload events.GWEventCharacterLoggedInPayload) error {
	if !g.stateTracker().recordGatewayLogin(payload) {
		return nil
	}

	if err := g.setGroupMemberOnlineStatus(payload.RealmID, payload.CharGUID, true); err != nil {
		return err
	}

	group, err := g.GroupByMemberGUID(context.Background(), payload.RealmID, payload.CharGUID)
	if err != nil {
		return err
	}

	return g.publishMemberStateCatchup(payload.RealmID, group, payload.CharGUID)
}

func (g *groupServiceImpl) HandleCharacterLoggedOut(payload events.GWEventCharacterLoggedOutPayload) error {
	if !g.stateTracker().recordGatewayLogout(payload) {
		return nil
	}

	if err := g.setGroupMemberOnlineStatus(payload.RealmID, payload.CharGUID, false); err != nil {
		return err
	}

	return nil
}

func (g *groupServiceImpl) HandleGatewayStarted(payload events.GWEventGatewayStartedPayload) error {
	var startedAt time.Time
	if payload.StartedAtMs != 0 {
		startedAt = time.UnixMilli(int64(payload.StartedAtMs))
	}

	members := g.stateTracker().onlineMembersForGateway(payload.RealmID, payload.GatewayID, startedAt)
	if len(members) == 0 {
		return nil
	}

	log.Info().
		Uint32("realmID", payload.RealmID).
		Str("gatewayID", payload.GatewayID).
		Uint64("startedAtMs", payload.StartedAtMs).
		Int("memberCount", len(members)).
		Msg("expiring gateway-owned group members after gateway start")

	nowMs := uint64(time.Now().UnixMilli())
	if payload.StartedAtMs != 0 {
		nowMs = payload.StartedAtMs
	}
	for _, memberGUID := range members {
		if err := g.setGroupMemberOnlineStatus(payload.RealmID, memberGUID, false); err != nil {
			return err
		}

		event, err := g.updateMemberState(context.Background(), memberStateUpdate{
			realmID:         payload.RealmID,
			sourceGatewayID: payload.GatewayID,
			snapshot: MemberStateSnapshot{
				MemberGUID:  memberGUID,
				Online:      false,
				TimestampMs: g.stateTracker().nextTimestampAfterLast(payload.RealmID, memberGUID, nowMs),
			},
		})
		if err != nil {
			return err
		}
		if event == nil {
			continue
		}
		if err := g.publishMemberStateBatch(&memberStateBatch{
			realmID:         event.realmID,
			groupID:         event.groupID,
			sourceGatewayID: event.sourceGatewayID,
			receivers:       append([]uint64(nil), event.receivers...),
			states:          []events.GroupMemberStateUpdate{event.state},
		}); err != nil {
			return err
		}
	}

	return nil
}

func (g *groupServiceImpl) setGroupMemberOnlineStatus(realmID uint32, player uint64, online bool) error {
	ctx := context.Background()

	if updater, ok := g.r.(groupMemberOnlineStatusUpdater); ok {
		group, err := updater.SetMemberOnlineStatus(ctx, realmID, player, online)
		if err != nil {
			return err
		}
		if group == nil {
			return nil
		}

		groupRealmID := groupHomeRealmID(realmID, group)
		player = groupPlayerGUID(groupRealmID, realmID, player)
		g.handleOfflineLeaderPromotion(ctx, groupRealmID, group)
		return g.ep.GroupMemberOnlineStatusChanged(&events.GroupEventGroupMemberOnlineStatusChangedPayload{
			ServiceID:     groupserver.ServiceID,
			RealmID:       groupRealmID,
			GroupID:       group.ID,
			MemberGUID:    player,
			IsOnline:      online,
			OnlineMembers: group.OnlineMemberGUIDs(),
		})
	}

	groupRealmID, groupID, err := g.groupRealmIDByPlayer(ctx, realmID, player)
	if err != nil {
		return err
	}

	if groupID == 0 {
		return nil
	}

	group, err := g.GroupByID(ctx, groupRealmID, groupID)
	if err != nil {
		return err
	}

	player = groupPlayerGUID(groupRealmID, realmID, player)
	member := group.MemberByGUID(player)
	if member == nil {
		return nil
	}

	member.IsOnline = online
	g.handleOfflineLeaderPromotion(ctx, groupRealmID, group)

	return g.ep.GroupMemberOnlineStatusChanged(&events.GroupEventGroupMemberOnlineStatusChangedPayload{
		ServiceID:     groupserver.ServiceID,
		RealmID:       groupRealmID,
		GroupID:       groupID,
		MemberGUID:    player,
		IsOnline:      online,
		OnlineMembers: group.OnlineMemberGUIDs(),
	})
}

func (g groupServiceImpl) getGroupWithLeader(ctx context.Context, realmID uint32, leaderGUID uint64) (*repo.Group, error) {
	group, err := g.GroupByMemberGUID(ctx, realmID, leaderGUID)
	if err != nil {
		return nil, fmt.Errorf("can't get group, err: %w", err)
	}

	if group == nil {
		return nil, ErrGroupNotFound
	}
	groupRealmID := groupHomeRealmID(realmID, group)
	leaderGUID = groupPlayerGUID(groupRealmID, realmID, leaderGUID)

	if !guid.SamePlayer(groupRealmID, group.LeaderGUID, groupRealmID, leaderGUID) {
		return nil, ErrNoPermissions
	}

	return group, nil
}

func (g groupServiceImpl) createGroup(ctx context.Context, realmID uint32, invite *repo.GroupInvite) error {
	invite.Inviter = groupPlayerGUID(realmID, invite.InviterRealmID, invite.Inviter)
	invite.Invitee = groupPlayerGUID(realmID, invite.InviteeRealmID, invite.Invitee)
	group := repo.Group{
		RealmID:          realmID,
		LeaderGUID:       invite.Inviter,
		LootMethod:       uint8(repo.LootTypeFreeForAll),
		LooterGUID:       invite.Inviter,
		LootThreshold:    uint8(repo.ItemQualityUncommon),
		TargetIcons:      [8]uint64{},
		GroupType:        repo.GroupTypeFlagsNormal,
		Difficulty:       0,
		RaidDifficulty:   0,
		MasterLooterGuid: invite.Inviter,
		Members: []repo.GroupMember{
			{
				RealmID:     guid.PlayerRealmIDOrDefault(realmID, invite.Inviter),
				MemberGUID:  invite.Inviter,
				MemberFlags: 0,
				MemberName:  invite.InviterName,
				IsOnline:    true,
				SubGroup:    0,
				Roles:       0,
			},
			{
				RealmID:     guid.PlayerRealmIDOrDefault(realmID, invite.Invitee),
				MemberGUID:  invite.Invitee,
				MemberFlags: 0,
				MemberName:  invite.InviteeName,
				IsOnline:    true,
				SubGroup:    0,
				Roles:       0,
			},
		},
	}

	err := g.r.Create(ctx, realmID, &group)
	if err != nil {
		return err
	}

	members := make([]events.GroupMember, len(group.Members))
	for i, member := range group.Members {
		members[i].MemberGUID = member.MemberGUID
		members[i].MemberFlags = member.MemberFlags
		members[i].MemberName = member.MemberName
		members[i].SubGroup = member.SubGroup
		members[i].IsOnline = member.IsOnline
		members[i].Roles = uint8(member.Roles)
	}

	err = g.ep.GroupCreated(&events.GroupEventGroupCreatedPayload{
		ServiceID:        groupserver.ServiceID,
		RealmID:          realmID,
		GroupID:          group.ID,
		LeaderGUID:       group.LeaderGUID,
		LootMethod:       group.LootMethod,
		LooterGUID:       group.LooterGUID,
		LootThreshold:    group.LootThreshold,
		GroupType:        uint8(group.GroupType),
		Difficulty:       group.Difficulty,
		RaidDifficulty:   group.RaidDifficulty,
		MasterLooterGuid: group.MasterLooterGuid,
		LfgDungeonEntry:  group.LfgDungeonEntry,
		Members:          members,
	})
	if err != nil {
		log.Error().Err(err).Msg("can't send group created event")
	}

	if err := g.publishMemberStateCatchupsForOnlineMembers(realmID, &group); err != nil {
		log.Error().
			Err(err).
			Str("reason", "group create").
			Msg("can't send group member-state catch-up")
	}

	return nil
}

func (g groupServiceImpl) addMember(ctx context.Context, realmID uint32, group *repo.Group, invite *repo.GroupInvite) error {
	if group == nil {
		return ErrGroupNotFound
	}

	if group.IsFull() {
		return ErrGroupFull
	}

	groupRealmID := groupHomeRealmID(realmID, group)
	invite.Invitee = groupPlayerGUID(groupRealmID, invite.InviteeRealmID, invite.Invitee)
	onlineMembers := append(group.OnlineMemberGUIDs(), invite.Invitee)

	member := repo.GroupMember{
		GroupID:     invite.GroupID,
		RealmID:     guid.PlayerRealmIDOrDefault(groupRealmID, invite.Invitee),
		MemberGUID:  invite.Invitee,
		MemberFlags: 0,
		MemberName:  invite.InviteeName,
		IsOnline:    true,
		SubGroup:    0,
		Roles:       0,
	}

	err := g.r.AddMember(ctx, groupRealmID, &member)
	if err != nil {
		return err
	}
	if group.MemberByGUID(invite.Invitee) == nil {
		group.Members = append(group.Members, member)
	}

	err = g.ep.MemberAdded(&events.GroupEventGroupMemberAddedPayload{
		ServiceID:     groupserver.ServiceID,
		RealmID:       groupRealmID,
		GroupID:       group.ID,
		MemberGUID:    invite.Invitee,
		MemberName:    invite.InviteeName,
		OnlineMembers: onlineMembers,
	})
	if err != nil {
		log.Error().Err(err).Msg("can't send group member added event")
	}

	if err := g.publishMemberStateCatchupsForOnlineMembers(groupRealmID, group); err != nil {
		log.Error().
			Err(err).
			Str("reason", "member add").
			Msg("can't send group member-state catch-up")
	}

	return nil
}

func (g *groupServiceImpl) disband(ctx context.Context, realmID uint32, group *repo.Group) error {
	players := group.OnlineMemberGUIDs()
	clearReadyCheckTimeout(realmID, group.ID)
	clearGroupOfflineLeaderPromotionTimers(realmID, group.ID)
	clearPendingSubGroupSwapsForGroup(realmID, group.ID)

	err := g.r.Delete(ctx, realmID, group.ID)
	if err != nil {
		return fmt.Errorf("can't delete group, err: %w", err)
	}
	g.stateTracker().clearGroup(realmID, group.Members)

	err = g.publishGroupDisband(realmID, group.ID, players)
	if err != nil {
		log.Error().Err(err).Msg("can't create GroupDisband event")
	}

	return nil
}

func (g groupServiceImpl) changeLeader(ctx context.Context, realmID uint32, group *repo.Group, newLeader uint64, needsEventUpdate bool) error {
	prevLeader := group.LeaderGUID
	realmID = groupHomeRealmID(realmID, group)
	newLeader = groupPlayerGUID(realmID, realmID, newLeader)
	clearGroupOfflineLeaderPromotionTimers(realmID, group.ID)

	group.LeaderGUID = newLeader
	if err := g.r.Update(ctx, realmID, group); err != nil {
		return fmt.Errorf("can't update group win a new leader, err: %w", err)
	}
	if needsEventUpdate {
		err := g.ep.LeaderChanged(&events.GroupEventGroupLeaderChangedPayload{
			ServiceID:      groupserver.ServiceID,
			RealmID:        realmID,
			GroupID:        group.ID,
			PreviousLeader: prevLeader,
			NewLeader:      newLeader,
			OnlineMembers:  group.OnlineMemberGUIDs(),
		})
		if err != nil {
			log.Error().Err(err).Msg("can't create LeaderChanged event")
		}
	}

	return nil
}

func (g groupServiceImpl) handleOfflineLeaderPromotion(ctx context.Context, realmID uint32, group *repo.Group) {
	if group == nil || group.LeaderGUID == 0 {
		return
	}

	leader := group.MemberByGUID(group.LeaderGUID)
	if leader == nil || leader.IsOnline {
		clearOfflineLeaderPromotionTimer(realmID, group.ID, group.LeaderGUID)
		return
	}

	if nextLeaderAfterOfflineLeader(group, group.LeaderGUID) == 0 {
		clearOfflineLeaderPromotionTimer(realmID, group.ID, group.LeaderGUID)
		return
	}

	sequence, queued := queueOfflineLeaderPromotionTimer(realmID, group.ID, group.LeaderGUID)
	if !queued {
		return
	}

	groupID := group.ID
	leaderGUID := group.LeaderGUID
	go func() {
		timer := time.NewTimer(offlineLeaderPromotionDelay)
		defer timer.Stop()

		<-timer.C
		if err := g.promoteOfflineLeaderIfStillNeeded(context.Background(), realmID, groupID, leaderGUID, sequence); err != nil {
			log.Error().Err(err).Uint32("realm_id", realmID).Uint("group_id", groupID).Uint64("leader_guid", leaderGUID).Msg("can't promote offline group leader")
		}
	}()
}

func (g groupServiceImpl) promoteOfflineLeaderIfStillNeeded(ctx context.Context, realmID uint32, groupID uint, leaderGUID uint64, sequence uint64) error {
	if !consumeOfflineLeaderPromotionTimer(realmID, groupID, leaderGUID, sequence) {
		return nil
	}

	group, err := g.GroupByID(ctx, realmID, groupID)
	if err != nil {
		return err
	}
	if group == nil || !guid.SamePlayer(realmID, group.LeaderGUID, realmID, leaderGUID) {
		return nil
	}

	leader := group.MemberByGUID(leaderGUID)
	if leader == nil || leader.IsOnline {
		return nil
	}

	newLeader := nextLeaderAfterOfflineLeader(group, leaderGUID)
	if newLeader == 0 {
		return nil
	}

	return g.changeLeader(ctx, realmID, group, newLeader, true)
}

func (g groupServiceImpl) StartReadyCheck(ctx context.Context, realmID uint32, leaderGUID uint64, durationMs uint32) error {
	if durationMs == 0 {
		durationMs = 35000
	}

	group, err := g.GroupByMemberGUID(ctx, realmID, leaderGUID)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}
	groupRealmID := groupHomeRealmID(realmID, group)
	leaderGUID = groupPlayerGUID(groupRealmID, realmID, leaderGUID)

	member := group.MemberByGUID(leaderGUID)
	if member == nil {
		return ErrGroupMemberNotFound
	}

	if !guid.SamePlayer(groupRealmID, group.LeaderGUID, groupRealmID, leaderGUID) && !member.IsAssistant() {
		return ErrNoPermissions
	}

	receivers := group.OnlineMemberGUIDs()

	if err := g.ep.GroupReadyCheckStarted(&events.GroupEventReadyCheckStartedPayload{
		ServiceID:  groupserver.ServiceID,
		RealmID:    groupRealmID,
		GroupID:    group.ID,
		LeaderGUID: leaderGUID,
		DurationMs: durationMs,
		Receivers:  receivers,
	}); err != nil {
		return err
	}

	for _, groupMember := range group.Members {
		if groupMember.IsOnline {
			continue
		}

		if err := g.ep.GroupReadyCheckMemberState(&events.GroupEventReadyCheckMemberStatePayload{
			ServiceID:  groupserver.ServiceID,
			RealmID:    groupRealmID,
			GroupID:    group.ID,
			MemberGUID: groupMember.MemberGUID,
			State:      0,
			Receivers:  receivers,
		}); err != nil {
			return err
		}
	}

	sequence := startReadyCheckTimeout(groupRealmID, group.ID)
	go func(realmID uint32, groupID uint, durationMs uint32, sequence uint64) {
		time.Sleep(time.Duration(durationMs) * time.Millisecond)
		if !consumeReadyCheckTimeout(realmID, groupID, sequence) {
			return
		}

		group, err := g.GroupByID(context.Background(), realmID, groupID)
		if err != nil {
			log.Error().Err(err).Uint32("realm_id", realmID).Uint("group_id", groupID).Msg("can't load group for ready check timeout")
			return
		}
		if group == nil {
			return
		}

		_ = g.ep.GroupReadyCheckFinished(&events.GroupEventReadyCheckFinishedPayload{
			ServiceID: groupserver.ServiceID,
			RealmID:   realmID,
			GroupID:   groupID,
			Receivers: group.OnlineMemberGUIDs(),
		})
	}(groupRealmID, group.ID, durationMs, sequence)

	return nil
}

func (g groupServiceImpl) SetReadyCheckMemberState(ctx context.Context, realmID uint32, memberGUID uint64, state uint8) error {
	if state > 2 {
		state = 2
	}

	group, err := g.GroupByMemberGUID(ctx, realmID, memberGUID)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}
	groupRealmID := groupHomeRealmID(realmID, group)
	memberGUID = groupPlayerGUID(groupRealmID, realmID, memberGUID)

	if group.MemberByGUID(memberGUID) == nil {
		return ErrGroupMemberNotFound
	}

	return g.ep.GroupReadyCheckMemberState(&events.GroupEventReadyCheckMemberStatePayload{
		ServiceID:  groupserver.ServiceID,
		RealmID:    groupRealmID,
		GroupID:    group.ID,
		MemberGUID: memberGUID,
		State:      state,
		Receivers:  group.OnlineMemberGUIDs(),
	})
}

func (g groupServiceImpl) FinishReadyCheck(ctx context.Context, realmID uint32, playerGUID uint64) error {
	group, err := g.GroupByMemberGUID(ctx, realmID, playerGUID)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}
	groupRealmID := groupHomeRealmID(realmID, group)
	playerGUID = groupPlayerGUID(groupRealmID, realmID, playerGUID)

	member := group.MemberByGUID(playerGUID)
	if member == nil {
		return ErrGroupMemberNotFound
	}

	if !guid.SamePlayer(groupRealmID, group.LeaderGUID, groupRealmID, playerGUID) && !member.IsAssistant() {
		return ErrNoPermissions
	}

	clearReadyCheckTimeout(groupRealmID, group.ID)

	return g.ep.GroupReadyCheckFinished(&events.GroupEventReadyCheckFinishedPayload{
		ServiceID: groupserver.ServiceID,
		RealmID:   groupRealmID,
		GroupID:   group.ID,
		Receivers: group.OnlineMemberGUIDs(),
	})
}

func (g groupServiceImpl) ChangeMemberSubGroup(ctx context.Context, realmID uint32, updaterGUID, memberGUID uint64, subGroup uint8) error {
	queueSwap := subGroup&subGroupSwapPendingFlag != 0
	subGroup &^= subGroupSwapPendingFlag

	if subGroup >= 8 {
		return ErrNoPermissions
	}

	group, err := g.GroupByMemberGUID(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}
	groupRealmID := groupHomeRealmID(realmID, group)
	updaterGUID = groupPlayerGUID(groupRealmID, realmID, updaterGUID)
	memberGUID = groupPlayerGUID(groupRealmID, realmID, memberGUID)

	updater := group.MemberByGUID(updaterGUID)
	if updater == nil {
		return ErrGroupMemberNotFound
	}

	if !guid.SamePlayer(groupRealmID, group.LeaderGUID, groupRealmID, updaterGUID) && !updater.IsAssistant() {
		return ErrNoPermissions
	}

	member := group.MemberByGUID(memberGUID)
	if member == nil {
		return ErrGroupMemberNotFound
	}

	if member.SubGroup == subGroup {
		return nil
	}

	if pending, ok := consumePendingSubGroupSwap(groupRealmID, group.ID, updaterGUID, memberGUID, member.SubGroup, subGroup); ok {
		firstMember := group.MemberByGUID(pending.memberGUID)
		if firstMember == nil {
			return ErrGroupMemberNotFound
		}
		if firstMember.SubGroup != pending.fromSubGroup {
			return ErrGroupFull
		}

		firstMember.SubGroup = pending.toSubGroup
		member.SubGroup = subGroup

		if err := g.r.UpdateMember(ctx, groupRealmID, firstMember); err != nil {
			return err
		}

		if err := g.r.UpdateMember(ctx, groupRealmID, member); err != nil {
			return err
		}

		receivers := group.OnlineMemberGUIDs()
		if err := g.ep.GroupMemberSubGroupChanged(&events.GroupEventMemberSubGroupChangedPayload{
			ServiceID:  groupserver.ServiceID,
			RealmID:    groupRealmID,
			GroupID:    group.ID,
			MemberGUID: firstMember.MemberGUID,
			SubGroup:   firstMember.SubGroup,
			Receivers:  receivers,
		}); err != nil {
			return err
		}

		return g.ep.GroupMemberSubGroupChanged(&events.GroupEventMemberSubGroupChangedPayload{
			ServiceID:  groupserver.ServiceID,
			RealmID:    groupRealmID,
			GroupID:    group.ID,
			MemberGUID: memberGUID,
			SubGroup:   subGroup,
			Receivers:  receivers,
		})
	}

	if subgroupMemberCount(group, subGroup, memberGUID) >= repo.MaxGroupSize {
		if queueSwap {
			return queuePendingSubGroupSwap(groupRealmID, group.ID, updaterGUID, memberGUID, member.SubGroup, subGroup)
		}

		return ErrGroupFull
	}

	member.SubGroup = subGroup

	if err := g.r.UpdateMember(ctx, groupRealmID, member); err != nil {
		return err
	}

	return g.ep.GroupMemberSubGroupChanged(&events.GroupEventMemberSubGroupChangedPayload{
		ServiceID:  groupserver.ServiceID,
		RealmID:    groupRealmID,
		GroupID:    group.ID,
		MemberGUID: memberGUID,
		SubGroup:   subGroup,
		Receivers:  group.OnlineMemberGUIDs(),
	})
}

func (g groupServiceImpl) SetMemberFlags(ctx context.Context, realmID uint32, updaterGUID, memberGUID uint64, flags, roles uint8) error {
	group, err := g.GroupByMemberGUID(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}
	groupRealmID := groupHomeRealmID(realmID, group)
	updaterGUID = groupPlayerGUID(groupRealmID, realmID, updaterGUID)
	memberGUID = groupPlayerGUID(groupRealmID, realmID, memberGUID)

	updater := group.MemberByGUID(updaterGUID)
	if updater == nil {
		return ErrGroupMemberNotFound
	}

	if !guid.SamePlayer(groupRealmID, group.LeaderGUID, groupRealmID, updaterGUID) && !updater.IsAssistant() {
		return ErrNoPermissions
	}

	member := group.MemberByGUID(memberGUID)
	if member == nil {
		return ErrGroupMemberNotFound
	}

	if member.MemberFlags&repo.MemberFlagAssistant != flags&repo.MemberFlagAssistant && !guid.SamePlayer(groupRealmID, group.LeaderGUID, groupRealmID, updaterGUID) {
		return ErrNoPermissions
	}

	receivers := group.OnlineMemberGUIDs()
	updatedMembers := make([]repo.GroupMember, 0, 3)

	uniqueFlags := flags & (repo.MemberFlagMainTank | repo.MemberFlagMainAssistant)
	if uniqueFlags != 0 {
		for i := range group.Members {
			if guid.SamePlayer(groupRealmID, group.Members[i].MemberGUID, groupRealmID, memberGUID) || group.Members[i].MemberFlags&uniqueFlags == 0 {
				continue
			}

			group.Members[i].MemberFlags &^= uniqueFlags
			if err := g.r.UpdateMember(ctx, groupRealmID, &group.Members[i]); err != nil {
				return err
			}

			updatedMembers = append(updatedMembers, group.Members[i])
		}
	}

	if member.MemberFlags != flags || uint8(member.Roles) != roles {
		member.MemberFlags = flags
		member.Roles = repo.RoleFlags(roles)

		if err := g.r.UpdateMember(ctx, groupRealmID, member); err != nil {
			return err
		}

		updatedMembers = append(updatedMembers, *member)
	}

	if len(updatedMembers) == 0 {
		return nil
	}

	for _, updatedMember := range updatedMembers {
		if err := g.ep.GroupMemberFlagsChanged(&events.GroupEventMemberFlagsChangedPayload{
			ServiceID:  groupserver.ServiceID,
			RealmID:    groupRealmID,
			GroupID:    group.ID,
			MemberGUID: updatedMember.MemberGUID,
			Flags:      updatedMember.MemberFlags,
			Roles:      uint8(updatedMember.Roles),
			Receivers:  receivers,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (g *groupServiceImpl) RegisterMaterializedLfgGroup(ctx context.Context, realmID uint32, groupID uint, leaderGUID uint64, groupType, difficulty, raidDifficulty uint8, members []MaterializedLfgGroupMember) error {
	if realmID == 0 || groupID == 0 || leaderGUID == 0 || len(members) == 0 {
		return nil
	}

	registrar, ok := g.r.(materializedLfgGroupRegistrar)
	if !ok {
		return fmt.Errorf("group repo does not support materialized LFG group registration")
	}

	group := &repo.Group{
		ID:             groupID,
		RealmID:        realmID,
		LeaderGUID:     groupPlayerGUID(realmID, realmID, leaderGUID),
		GroupType:      repo.GroupTypeFlags(groupType),
		Difficulty:     difficulty,
		RaidDifficulty: raidDifficulty,
		Members:        make([]repo.GroupMember, 0, len(members)),
	}

	seenMembers := make(map[uint64]struct{}, len(members))
	for _, member := range members {
		if member.PlayerGUID == 0 || member.RealmID == 0 {
			continue
		}

		memberGUID := groupPlayerGUID(realmID, member.RealmID, member.PlayerGUID)
		if _, ok := seenMembers[memberGUID]; ok {
			continue
		}
		seenMembers[memberGUID] = struct{}{}

		memberOnline := member.Online
		if !memberOnline && g.stateTracker().memberHasOnlineEvidence(realmID, memberGUID) {
			memberOnline = true
			if event := groupstatetrace.Event(nil, "groupserver.materialized_lfg_member_online_recovered", memberGUID); event != nil {
				event.
					Uint32("realmID", guid.PlayerRealmIDOrDefault(realmID, memberGUID)).
					Uint64("memberGUID", memberGUID).
					Uint("groupID", groupID).
					Msg(groupstatetrace.Message)
			}
		}

		group.Members = append(group.Members, repo.GroupMember{
			GroupID:     groupID,
			RealmID:     guid.PlayerRealmIDOrDefault(realmID, memberGUID),
			MemberGUID:  memberGUID,
			MemberFlags: member.Flags,
			MemberName:  member.Name,
			IsOnline:    memberOnline,
			SubGroup:    member.SubGroup,
			Roles:       repo.RoleFlags(member.Roles),
		})
	}

	if len(group.Members) == 0 {
		return nil
	}

	if group.MemberByGUID(group.LeaderGUID) == nil {
		group.LeaderGUID = group.Members[0].MemberGUID
	}

	oldGroups, err := g.materializedLfgSupersededGroups(ctx, realmID, group)
	if err != nil {
		return err
	}
	canonicalGroup := materializedLfgCanonicalAcceptedGroup(realmID, group, oldGroups)
	if canonicalGroup != nil {
		realmID = canonicalGroup.realmID
		group.ID = canonicalGroup.group.ID
		group.RealmID = canonicalGroup.realmID
		group.LfgDungeonEntry = canonicalGroup.group.LfgDungeonEntry
		group.LeaderGUID = groupPlayerGUID(realmID, guid.PlayerRealmIDOrDefault(realmID, group.LeaderGUID), guid.PlayerLowGUID(group.LeaderGUID))
		for i := range group.Members {
			memberRealmID := guid.PlayerRealmIDOrDefault(realmID, group.Members[i].MemberGUID)
			group.Members[i].GroupID = group.ID
			group.Members[i].RealmID = memberRealmID
			group.Members[i].MemberGUID = groupPlayerGUID(realmID, memberRealmID, guid.PlayerLowGUID(group.Members[i].MemberGUID))
		}
	}
	for _, oldGroup := range oldGroups {
		if oldGroup.group == nil {
			continue
		}
		if canonicalGroup != nil && oldGroup.realmID == canonicalGroup.realmID && oldGroup.group.ID == canonicalGroup.group.ID {
			continue
		}
		if err := g.removeAcceptedLfgOldGroup(ctx, oldGroup.realmID, oldGroup.group); err != nil {
			return err
		}
	}

	if err := registrar.RegisterMaterializedLfgGroup(ctx, realmID, group); err != nil {
		return err
	}

	log.Debug().
		Uint32("realmID", realmID).
		Uint("groupID", groupID).
		Uint("canonicalGroupID", group.ID).
		Uint64("leaderGUID", group.LeaderGUID).
		Uint8("groupType", groupType).
		Int("memberCount", len(group.Members)).
		Msg("registered materialized LFG group for clustered member state")

	if err := g.publishMemberStateCatchupsForOnlineMembers(realmID, group); err != nil {
		log.Error().
			Err(err).
			Str("reason", "materialized LFG group registration").
			Msg("can't send group member-state catch-up")
	}

	return nil
}

func materializedLfgCanonicalAcceptedGroup(realmID uint32, group *repo.Group, oldGroups []acceptedLfgExistingGroup) *acceptedLfgExistingGroup {
	if group == nil || len(group.Members) == 0 {
		return nil
	}

	for i := range oldGroups {
		oldGroup := oldGroups[i]
		if materializedLfgMatchesAcceptedGroup(realmID, group, oldGroup.realmID, oldGroup.group) {
			return &oldGroups[i]
		}
	}

	return nil
}

func materializedLfgMatchesAcceptedGroup(materializedRealmID uint32, materializedGroup *repo.Group, acceptedRealmID uint32, acceptedGroup *repo.Group) bool {
	if materializedGroup == nil || acceptedGroup == nil || acceptedGroup.ID == 0 {
		return false
	}
	if acceptedGroup.GroupType&repo.GroupTypeFlagsLFG == 0 {
		return false
	}
	if len(materializedGroup.Members) != len(acceptedGroup.Members) {
		return false
	}

	leaderRealmID := guid.PlayerRealmIDOrDefault(materializedRealmID, materializedGroup.LeaderGUID)
	leaderGUID := groupPlayerGUID(acceptedRealmID, leaderRealmID, guid.PlayerLowGUID(materializedGroup.LeaderGUID))
	if acceptedGroup.MemberByGUID(leaderGUID) == nil {
		return false
	}

	for _, member := range materializedGroup.Members {
		memberRealmID := guid.PlayerRealmIDOrDefault(materializedRealmID, member.MemberGUID)
		memberGUID := groupPlayerGUID(acceptedRealmID, memberRealmID, guid.PlayerLowGUID(member.MemberGUID))
		if acceptedGroup.MemberByGUID(memberGUID) == nil {
			return false
		}
	}

	return true
}

func (g *groupServiceImpl) materializedLfgSupersededGroups(ctx context.Context, realmID uint32, group *repo.Group) ([]acceptedLfgExistingGroup, error) {
	if group == nil || group.ID == 0 {
		return nil, nil
	}

	seenGroups := map[acceptedLfgGroupKey]struct{}{}
	oldGroups := make([]acceptedLfgExistingGroup, 0)
	currentGroupKey := acceptedLfgGroupKey{realmID: realmID, groupID: group.ID}

	for _, member := range group.Members {
		if member.MemberGUID == 0 {
			continue
		}

		memberRealmID := guid.PlayerRealmIDOrDefault(realmID, member.MemberGUID)
		memberGUID := guid.PlayerLowGUID(member.MemberGUID)
		groupRealmID, groupID, err := g.groupRealmIDByPlayer(ctx, memberRealmID, memberGUID)
		if err != nil || groupID == 0 {
			return nil, err
		}

		key := acceptedLfgGroupKey{realmID: groupRealmID, groupID: groupID}
		if key == currentGroupKey {
			continue
		}
		if _, ok := seenGroups[key]; ok {
			continue
		}
		seenGroups[key] = struct{}{}

		oldGroup, err := g.GroupByID(ctx, groupRealmID, groupID)
		if err != nil {
			return nil, err
		}
		if oldGroup == nil {
			continue
		}

		oldGroups = append(oldGroups, acceptedLfgExistingGroup{realmID: groupRealmID, group: oldGroup})
	}

	return oldGroups, nil
}

func (g *groupServiceImpl) RegisterAcceptedLfgGroup(ctx context.Context, realmID, proposalID, dungeonEntry, leaderRealmID uint32, leaderGUID uint64, crossRealm bool, members []AcceptedLfgGroupMember) (uint, error) {
	groupRealmID := leaderRealmID
	if groupRealmID == 0 {
		groupRealmID = realmID
	}
	if groupRealmID == 0 || leaderGUID == 0 || len(members) == 0 {
		return 0, nil
	}

	registrar, ok := g.r.(acceptedLfgGroupRegistrar)
	if !ok {
		return 0, fmt.Errorf("group repo does not support accepted LFG group registration")
	}

	baseRealmID, baseGroup, oldGroups, err := g.acceptedLfgBaseGroup(ctx, groupRealmID, leaderRealmID, leaderGUID, members)
	if err != nil {
		return 0, err
	}
	if baseGroup != nil {
		groupRealmID = baseRealmID
	}
	for _, oldGroup := range oldGroups {
		if oldGroup.group == nil {
			continue
		}
		if err := g.removeAcceptedLfgOldGroup(ctx, oldGroup.realmID, oldGroup.group); err != nil {
			return 0, err
		}
	}

	namesByGUID, onlineByGUID, err := g.acceptedLfgMemberNames(ctx, groupRealmID, members)
	if err != nil {
		return 0, err
	}

	group := acceptedLfgGroupFromMembers(groupRealmID, leaderRealmID, leaderGUID, dungeonEntry, baseGroup, members, namesByGUID, onlineByGUID)
	if group == nil || len(group.Members) == 0 {
		return 0, nil
	}

	if err := registrar.RegisterAcceptedLfgGroup(ctx, groupRealmID, group); err != nil {
		return 0, err
	}
	if group.ID == 0 {
		return 0, fmt.Errorf("accepted LFG group registration did not allocate a group id")
	}

	log.Debug().
		Uint32("realmID", realmID).
		Uint32("groupRealmID", groupRealmID).
		Uint32("proposalID", proposalID).
		Uint32("dungeonEntry", dungeonEntry).
		Uint("groupID", group.ID).
		Uint64("leaderGUID", group.LeaderGUID).
		Bool("crossRealm", crossRealm).
		Int("memberCount", len(group.Members)).
		Msg("registered accepted LFG group for clustered travel state")

	if err := g.publishGroupCreated(groupRealmID, group); err != nil {
		log.Error().
			Err(err).
			Uint32("realmID", groupRealmID).
			Uint("groupID", group.ID).
			Msg("can't send accepted LFG group created event")
	}

	return group.ID, nil
}

type acceptedLfgExistingGroup struct {
	realmID uint32
	group   *repo.Group
}

type acceptedLfgGroupKey struct {
	realmID uint32
	groupID uint
}

func (g *groupServiceImpl) acceptedLfgBaseGroup(ctx context.Context, fallbackRealmID, leaderRealmID uint32, leaderGUID uint64, members []AcceptedLfgGroupMember) (uint32, *repo.Group, []acceptedLfgExistingGroup, error) {
	seenGroups := map[acceptedLfgGroupKey]struct{}{}
	groups := make([]acceptedLfgExistingGroup, 0, len(members))
	var leaderGroupKey acceptedLfgGroupKey
	leaderHasGroup := false
	leaderIdentityRealmID := leaderRealmID
	if leaderIdentityRealmID == 0 {
		leaderIdentityRealmID = fallbackRealmID
	}

	tryPlayer := func(playerRealmID uint32, playerGUID uint64) (*acceptedLfgGroupKey, error) {
		if playerRealmID == 0 {
			playerRealmID = fallbackRealmID
		}
		if playerRealmID == 0 || playerGUID == 0 {
			return nil, nil
		}

		groupRealmID, groupID, err := g.groupRealmIDByPlayer(ctx, playerRealmID, playerGUID)
		if err != nil || groupID == 0 {
			return nil, err
		}
		key := acceptedLfgGroupKey{realmID: groupRealmID, groupID: groupID}
		if _, ok := seenGroups[key]; ok {
			return &key, nil
		}
		seenGroups[key] = struct{}{}

		group, err := g.GroupByID(ctx, groupRealmID, groupID)
		if err != nil || group == nil {
			return nil, err
		}
		groups = append(groups, acceptedLfgExistingGroup{realmID: groupRealmID, group: group})
		return &key, nil
	}

	if key, err := tryPlayer(leaderRealmID, leaderGUID); err != nil {
		return 0, nil, nil, err
	} else if key != nil {
		leaderGroupKey = *key
		leaderHasGroup = true
	}
	for _, member := range members {
		memberRealmID := acceptedLfgMemberRealmID(fallbackRealmID, member)
		key, err := tryPlayer(memberRealmID, member.PlayerGUID)
		if err != nil {
			return 0, nil, nil, err
		}
		if key != nil && !leaderHasGroup && guid.SamePlayer(leaderIdentityRealmID, leaderGUID, memberRealmID, member.PlayerGUID) {
			leaderGroupKey = *key
			leaderHasGroup = true
		}
	}

	if len(groups) == 0 {
		return fallbackRealmID, nil, nil, nil
	}
	if !leaderHasGroup {
		return fallbackRealmID, nil, groups, nil
	}

	oldGroups := make([]acceptedLfgExistingGroup, 0, len(groups)-1)
	var baseGroup *repo.Group
	baseRealmID := fallbackRealmID
	for _, existing := range groups {
		if existing.group == nil {
			continue
		}
		key := acceptedLfgGroupKey{realmID: existing.realmID, groupID: existing.group.ID}
		if key == leaderGroupKey {
			baseRealmID = existing.realmID
			baseGroup = existing.group
			continue
		}
		oldGroups = append(oldGroups, existing)
	}

	if baseGroup == nil {
		return fallbackRealmID, nil, groups, nil
	}
	return baseRealmID, baseGroup, oldGroups, nil
}

func (g *groupServiceImpl) acceptedLfgMemberNames(ctx context.Context, groupRealmID uint32, members []AcceptedLfgGroupMember) (map[uint64]string, map[uint64]bool, error) {
	namesByGUID := map[uint64]string{}
	onlineByGUID := map[uint64]bool{}
	if g.charClient == nil {
		return namesByGUID, onlineByGUID, nil
	}

	memberGUIDs := make([]uint64, 0, len(members))
	seen := make(map[uint64]struct{}, len(members))
	for _, member := range members {
		memberRealmID := acceptedLfgMemberRealmID(groupRealmID, member)
		if member.PlayerGUID == 0 || memberRealmID == 0 {
			continue
		}
		memberGUID := groupPlayerGUID(groupRealmID, memberRealmID, member.PlayerGUID)
		if _, ok := seen[memberGUID]; ok {
			continue
		}
		seen[memberGUID] = struct{}{}
		memberGUIDs = append(memberGUIDs, memberGUID)
	}

	characters, err := g.shortOnlineCharactersForGroup(ctx, groupRealmID, memberGUIDs)
	if err != nil {
		return nil, nil, err
	}
	for _, character := range characters {
		if character == nil || character.GetRealmID() == 0 || character.GetCharGUID() == 0 {
			continue
		}
		memberGUID := groupPlayerGUID(groupRealmID, character.GetRealmID(), character.GetCharGUID())
		namesByGUID[memberGUID] = character.GetCharName()
		onlineByGUID[memberGUID] = character.GetIsOnline()
	}

	return namesByGUID, onlineByGUID, nil
}

func acceptedLfgGroupFromMembers(groupRealmID, leaderRealmID uint32, leaderGUID uint64, dungeonEntry uint32, baseGroup *repo.Group, members []AcceptedLfgGroupMember, namesByGUID map[uint64]string, onlineByGUID map[uint64]bool) *repo.Group {
	leader := groupPlayerGUID(groupRealmID, leaderRealmID, leaderGUID)
	group := &repo.Group{
		RealmID:          groupRealmID,
		LeaderGUID:       leader,
		LootMethod:       uint8(repo.LootTypeNeedBeforeGreed),
		LooterGUID:       leader,
		LootThreshold:    uint8(repo.ItemQualityUncommon),
		GroupType:        repo.GroupTypeFlagsLFG | repo.GroupTypeFlagsLFGRestricted,
		MasterLooterGuid: 0,
		LfgDungeonEntry:  dungeonEntry,
	}
	existingMembers := map[uint64]repo.GroupMember{}
	if baseGroup != nil {
		group = cloneGroup(baseGroup)
		group.RealmID = groupRealmID
		group.LeaderGUID = leader
		group.GroupType |= repo.GroupTypeFlagsLFG
		group.LfgDungeonEntry = dungeonEntry
		for _, member := range baseGroup.Members {
			existingMembers[member.MemberGUID] = member
		}
	}

	seen := make(map[uint64]struct{}, len(members))
	group.Members = make([]repo.GroupMember, 0, len(members))
	for _, member := range members {
		memberRealmID := acceptedLfgMemberRealmID(groupRealmID, member)
		if member.PlayerGUID == 0 || memberRealmID == 0 {
			continue
		}

		memberGUID := groupPlayerGUID(groupRealmID, memberRealmID, member.PlayerGUID)
		if _, ok := seen[memberGUID]; ok {
			continue
		}
		seen[memberGUID] = struct{}{}

		role := member.AssignedRole
		if role == 0 {
			role = member.SelectedRoles
		}

		memberName := namesByGUID[memberGUID]
		if memberName == "" {
			if existingMember, ok := existingMembers[memberGUID]; ok {
				memberName = existingMember.MemberName
			}
		}

		online := true
		if knownOnline, ok := onlineByGUID[memberGUID]; ok {
			online = knownOnline
		}

		group.Members = append(group.Members, repo.GroupMember{
			GroupID:    group.ID,
			RealmID:    guid.PlayerRealmIDOrDefault(groupRealmID, memberGUID),
			MemberGUID: memberGUID,
			MemberName: memberName,
			IsOnline:   online,
			Roles:      repo.RoleFlags(role),
		})
	}

	if len(group.Members) == 0 {
		return nil
	}
	if group.MemberByGUID(group.LeaderGUID) == nil {
		group.LeaderGUID = group.Members[0].MemberGUID
	}
	if group.LooterGUID == 0 {
		group.LooterGUID = group.LeaderGUID
	}

	return group
}

func acceptedLfgMemberRealmID(fallbackRealmID uint32, member AcceptedLfgGroupMember) uint32 {
	if member.RealmID != 0 {
		return member.RealmID
	}
	if member.QueueLeaderRealmID != 0 {
		return member.QueueLeaderRealmID
	}
	return fallbackRealmID
}

func (g *groupServiceImpl) removeAcceptedLfgOldGroup(ctx context.Context, realmID uint32, group *repo.Group) error {
	if group == nil {
		return nil
	}

	players := group.OnlineMemberGUIDs()
	clearReadyCheckTimeout(realmID, group.ID)
	clearGroupOfflineLeaderPromotionTimers(realmID, group.ID)
	clearPendingSubGroupSwapsForGroup(realmID, group.ID)

	if err := g.r.Delete(ctx, realmID, group.ID); err != nil {
		return fmt.Errorf("can't delete accepted LFG superseded group, err: %w", err)
	}

	if err := g.publishGroupDisband(realmID, group.ID, players); err != nil {
		log.Error().Err(err).Msg("can't create accepted LFG superseded GroupDisband event")
	}

	return nil
}

func (g *groupServiceImpl) publishGroupDisband(realmID uint32, groupID uint, onlineMembers []uint64) error {
	return g.ep.GroupDisband(&events.GroupEventGroupDisbandPayload{
		ServiceID:     groupserver.ServiceID,
		RealmID:       realmID,
		GroupID:       groupID,
		OnlineMembers: onlineMembers,
	})
}

func (g *groupServiceImpl) publishGroupCreated(realmID uint32, group *repo.Group) error {
	members := make([]events.GroupMember, len(group.Members))
	for i, member := range group.Members {
		members[i].MemberGUID = member.MemberGUID
		members[i].MemberFlags = member.MemberFlags
		members[i].MemberName = member.MemberName
		members[i].SubGroup = member.SubGroup
		members[i].IsOnline = member.IsOnline
		members[i].Roles = uint8(member.Roles)
	}

	return g.ep.GroupCreated(&events.GroupEventGroupCreatedPayload{
		ServiceID:        groupserver.ServiceID,
		RealmID:          realmID,
		GroupID:          group.ID,
		LeaderGUID:       group.LeaderGUID,
		LootMethod:       group.LootMethod,
		LooterGUID:       group.LooterGUID,
		LootThreshold:    group.LootThreshold,
		GroupType:        uint8(group.GroupType),
		Difficulty:       group.Difficulty,
		RaidDifficulty:   group.RaidDifficulty,
		MasterLooterGuid: group.MasterLooterGuid,
		LfgDungeonEntry:  group.LfgDungeonEntry,
		Members:          members,
	})
}

func (g *groupServiceImpl) UpdateMemberState(ctx context.Context, realmID uint32, memberGUID uint64, online bool, level, class uint8, zoneID, mapID uint32, health, maxHealth uint32, powerType uint8, power, maxPower uint32, instanceID *uint32) error {
	event, err := g.updateMemberState(ctx, memberStateUpdate{
		realmID: realmID,
		snapshot: MemberStateSnapshot{
			MemberGUID: memberGUID,
			Online:     online,
			Level:      level,
			Class:      class,
			ZoneID:     zoneID,
			MapID:      mapID,
			Health:     health,
			MaxHealth:  maxHealth,
			PowerType:  powerType,
			Power:      power,
			MaxPower:   maxPower,
			InstanceID: instanceID,
		},
	})
	if err != nil || event == nil {
		return err
	}

	return g.publishMemberStateBatch(&memberStateBatch{
		realmID:             event.realmID,
		groupID:             event.groupID,
		sourceGatewayID:     event.sourceGatewayID,
		sourceWorldserverID: event.sourceWorldserverID,
		receivers:           event.receivers,
		states:              []events.GroupMemberStateUpdate{event.state},
	})
}

func (g *groupServiceImpl) updateMemberState(ctx context.Context, update memberStateUpdate) (*memberStateEvent, error) {
	snapshot := update.snapshot

	if snapshot.MaxHealth > 0 && snapshot.Health > snapshot.MaxHealth {
		snapshot.Health = snapshot.MaxHealth
	}
	if snapshot.MaxPower > 0 && snapshot.Power > snapshot.MaxPower {
		snapshot.Power = snapshot.MaxPower
	}
	if event := groupstatetrace.Event(nil, "groupserver.member_state.received", snapshot.MemberGUID); event != nil {
		traceMemberStateSnapshot(event, update.realmID, snapshot).
			Str("sourceGatewayID", update.sourceGatewayID).
			Str("sourceWorldserverID", update.sourceWorldserverID).
			Msg(groupstatetrace.Message)
	}
	snapshot = normalizeFixedClassMemberPower(snapshot)
	if isInvalidOnlineTransitionSnapshot(snapshot) {
		if event := groupstatetrace.Event(nil, "groupserver.member_state.drop_invalid_transition", snapshot.MemberGUID); event != nil {
			traceMemberStateSnapshot(event, update.realmID, snapshot).
				Str("sourceGatewayID", update.sourceGatewayID).
				Str("sourceWorldserverID", update.sourceWorldserverID).
				Msg(groupstatetrace.Message)
		}
		return nil, nil
	}
	group, err := g.GroupByMemberGUID(ctx, update.realmID, snapshot.MemberGUID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		g.recordUngroupedMemberState(update, snapshot)
		return nil, nil
	}
	groupRealmID := groupHomeRealmID(update.realmID, group)
	snapshot.MemberGUID = groupPlayerGUID(groupRealmID, update.realmID, snapshot.MemberGUID)

	member := group.MemberByGUID(snapshot.MemberGUID)
	if member == nil {
		return nil, nil
	}
	snapshot.MemberGUID = member.MemberGUID

	if update.sourceGatewayID == "" && update.sourceWorldserverID == "" && snapshot.TimestampMs == 0 && g.stateTracker().hasTimestamp(update.realmID, snapshot.MemberGUID) {
		if event := groupstatetrace.Event(nil, "groupserver.member_state.drop_source_less_untimestamped", snapshot.MemberGUID); event != nil {
			traceMemberStateSnapshot(event, update.realmID, snapshot).
				Msg(groupstatetrace.Message)
		}
		return nil, nil
	}
	if isAuraOnlyMemberStateSnapshot(snapshot) && !g.stateTracker().hasOnlineState(update.realmID, snapshot.MemberGUID) {
		g.stateTracker().recordMemberPlacement(update, snapshot)
		if event := groupstatetrace.Event(nil, "groupserver.member_state.drop_initial_aura_only", snapshot.MemberGUID); event != nil {
			traceMemberStateSnapshot(event, update.realmID, snapshot).
				Str("sourceGatewayID", update.sourceGatewayID).
				Str("sourceWorldserverID", update.sourceWorldserverID).
				Msg(groupstatetrace.Message)
		}
		return nil, nil
	}

	var accepted bool
	snapshot.TimestampMs, accepted = g.stateTracker().acceptTimestamp(update.realmID, snapshot.MemberGUID, snapshot.TimestampMs, snapshot.AurasKnown)
	if !accepted {
		if event := groupstatetrace.Event(nil, "groupserver.member_state.drop_stale", snapshot.MemberGUID); event != nil {
			traceMemberStateSnapshot(event, update.realmID, snapshot).
				Str("sourceGatewayID", update.sourceGatewayID).
				Str("sourceWorldserverID", update.sourceWorldserverID).
				Msg(groupstatetrace.Message)
		}
		return nil, nil
	}

	snapshot = g.stateTracker().preserveKnownVitals(update.realmID, snapshot)
	snapshot = normalizeFixedClassMemberPower(snapshot)
	if snapshot.MaxHealth > 0 && snapshot.Health > snapshot.MaxHealth {
		snapshot.Health = snapshot.MaxHealth
	}
	if snapshot.MaxPower > 0 && snapshot.Power > snapshot.MaxPower {
		snapshot.Power = snapshot.MaxPower
	}
	if event := groupstatetrace.Event(nil, "groupserver.member_state.accepted", snapshot.MemberGUID); event != nil {
		traceMemberStateSnapshot(event, update.realmID, snapshot).
			Uint("groupID", group.ID).
			Str("sourceGatewayID", update.sourceGatewayID).
			Str("sourceWorldserverID", update.sourceWorldserverID).
			Msg(groupstatetrace.Message)
	}

	if member.IsOnline != snapshot.Online {
		member.IsOnline = snapshot.Online

		if err := g.r.UpdateMember(ctx, groupRealmID, member); err != nil {
			return nil, err
		}

		g.handleOfflineLeaderPromotion(ctx, groupRealmID, group)
	}

	g.stateTracker().recordMemberState(update, snapshot)

	event := &memberStateEvent{
		realmID:             groupRealmID,
		groupID:             group.ID,
		sourceGatewayID:     update.sourceGatewayID,
		sourceWorldserverID: update.sourceWorldserverID,
		receivers:           group.OnlineMemberGUIDs(),
		state:               memberStateSnapshotToUpdate(snapshot),
	}

	suppressed, suppressionReason := g.localMemberStateFanoutSuppressionDecision(event)
	if traceEvent := groupstatetrace.Event(nil, "groupserver.member_state.fanout_decision", event.state.MemberGUID); traceEvent != nil {
		traceEvent.
			Uint32("realmID", event.realmID).
			Uint("groupID", event.groupID).
			Uint64("memberGUID", event.state.MemberGUID).
			Str("sourceGatewayID", event.sourceGatewayID).
			Str("sourceWorldserverID", event.sourceWorldserverID).
			Bool("suppressed", suppressed).
			Str("reason", suppressionReason).
			Int("receiverCount", len(event.receivers)).
			Str("receivers", memberStateReceiversKey(event.receivers)).
			Msg(groupstatetrace.Message)
	}
	if suppressed {
		return nil, nil
	}

	return event, nil
}

func (g *groupServiceImpl) recordUngroupedMemberState(update memberStateUpdate, snapshot MemberStateSnapshot) {
	hasSource := update.sourceGatewayID != "" || update.sourceWorldserverID != ""
	if snapshot.Online && !hasSource {
		g.stateTracker().recordMemberPlacement(update, snapshot)
		return
	}

	if update.sourceGatewayID == "" && update.sourceWorldserverID == "" && snapshot.TimestampMs == 0 && g.stateTracker().hasTimestamp(update.realmID, snapshot.MemberGUID) {
		return
	}
	if isAuraOnlyMemberStateSnapshot(snapshot) && !g.stateTracker().hasOnlineState(update.realmID, snapshot.MemberGUID) {
		g.stateTracker().recordMemberPlacement(update, snapshot)
		return
	}

	var accepted bool
	snapshot.TimestampMs, accepted = g.stateTracker().acceptTimestamp(update.realmID, snapshot.MemberGUID, snapshot.TimestampMs, snapshot.AurasKnown)
	if !accepted {
		return
	}

	snapshot = g.stateTracker().preserveKnownVitals(update.realmID, snapshot)
	snapshot = normalizeFixedClassMemberPower(snapshot)
	if snapshot.MaxHealth > 0 && snapshot.Health > snapshot.MaxHealth {
		snapshot.Health = snapshot.MaxHealth
	}
	if snapshot.MaxPower > 0 && snapshot.Power > snapshot.MaxPower {
		snapshot.Power = snapshot.MaxPower
	}

	g.stateTracker().recordMemberState(update, snapshot)
}

func memberStateSnapshotToUpdate(snapshot MemberStateSnapshot) events.GroupMemberStateUpdate {
	return events.GroupMemberStateUpdate{
		MemberGUID: snapshot.MemberGUID,
		Online:     snapshot.Online,
		Level:      snapshot.Level,
		Class:      snapshot.Class,
		ZoneID:     snapshot.ZoneID,
		MapID:      snapshot.MapID,
		Health:     snapshot.Health,
		MaxHealth:  snapshot.MaxHealth,
		PowerType:  snapshot.PowerType,
		Power:      snapshot.Power,
		MaxPower:   snapshot.MaxPower,
		AurasKnown: snapshot.AurasKnown,
		Auras:      memberAurasToEvent(normalizeMemberAuras(snapshot.Auras)),
		DeadKnown:  snapshot.Dead != nil,
		Dead:       boolPtrValue(snapshot.Dead),
		GhostKnown: snapshot.Ghost != nil,
		Ghost:      boolPtrValue(snapshot.Ghost),
	}
}

func isInvalidOnlineTransitionSnapshot(snapshot MemberStateSnapshot) bool {
	return snapshot.Online && snapshot.MapID == ^uint32(0)
}

func (g *groupServiceImpl) localMemberStateFanoutSuppressionDecision(event *memberStateEvent) (bool, string) {
	if event == nil || event.sourceWorldserverID == "" || !event.state.Online || len(event.receivers) == 0 {
		return false, "not-eligible"
	}

	now := time.Now()
	tracker := g.stateTracker()
	source, ok := tracker.freshPlacement(event.realmID, event.state.MemberGUID, now, memberPlacementFreshness)
	if !ok || !sameFreshLocalPlacement(source, event.sourceGatewayID, event.sourceWorldserverID, event.state.MapID) {
		return false, "source-placement-not-fresh-local"
	}

	mapID := MapID(int(event.state.MapID))
	requireInstance := mapID.IsDungeon() || mapID.IsRaid()
	if requireInstance && (!source.instanceKnown || source.instanceID == 0) {
		return false, "source-instance-unknown"
	}

	for _, receiverGUID := range event.receivers {
		receiver, ok := tracker.freshPlacement(event.realmID, receiverGUID, now, memberPlacementFreshness)
		if !ok || !sameFreshLocalPlacement(receiver, "", event.sourceWorldserverID, event.state.MapID) {
			return false, "receiver-placement-not-fresh-local"
		}
		if requireInstance && (!receiver.instanceKnown || receiver.instanceID == 0 || receiver.instanceID != source.instanceID) {
			return false, "receiver-instance-mismatch"
		}
	}

	return true, "same-fresh-local-owner"
}

func sameFreshLocalPlacement(placement memberPlacement, gatewayID, worldserverID string, mapID uint32) bool {
	if !placement.online || placement.worldserverID == "" || placement.worldserverID != worldserverID || placement.mapID != mapID {
		return false
	}

	return gatewayID == "" || placement.gatewayID == "" || placement.gatewayID == gatewayID
}

func (g *groupServiceImpl) BulkUpdateMemberStates(ctx context.Context, realmID uint32, sourceGatewayID, sourceWorldserverID string, snapshots []MemberStateSnapshot) error {
	batches := map[memberStateBatchKey]*memberStateBatch{}

	for _, snapshot := range snapshots {
		if snapshot.MemberGUID == 0 {
			continue
		}

		event, err := g.updateMemberState(ctx, memberStateUpdate{
			realmID:             realmID,
			sourceGatewayID:     sourceGatewayID,
			sourceWorldserverID: sourceWorldserverID,
			snapshot:            snapshot,
		})
		if err != nil {
			return err
		}
		if event == nil {
			continue
		}

		key := memberStateBatchKey{
			realmID:             event.realmID,
			groupID:             event.groupID,
			sourceGatewayID:     event.sourceGatewayID,
			sourceWorldserverID: event.sourceWorldserverID,
			receiversKey:        memberStateReceiversKey(event.receivers),
		}

		batch := batches[key]
		if batch == nil {
			batch = &memberStateBatch{
				realmID:             event.realmID,
				groupID:             event.groupID,
				sourceGatewayID:     event.sourceGatewayID,
				sourceWorldserverID: event.sourceWorldserverID,
				receivers:           append([]uint64(nil), event.receivers...),
			}
			batches[key] = batch
		}

		batch.states = append(batch.states, event.state)
	}

	keys := make([]memberStateBatchKey, 0, len(batches))
	for key := range batches {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].realmID != keys[j].realmID {
			return keys[i].realmID < keys[j].realmID
		}
		if keys[i].groupID != keys[j].groupID {
			return keys[i].groupID < keys[j].groupID
		}
		if keys[i].sourceGatewayID != keys[j].sourceGatewayID {
			return keys[i].sourceGatewayID < keys[j].sourceGatewayID
		}
		if keys[i].sourceWorldserverID != keys[j].sourceWorldserverID {
			return keys[i].sourceWorldserverID < keys[j].sourceWorldserverID
		}
		return keys[i].receiversKey < keys[j].receiversKey
	})

	for _, key := range keys {
		if err := g.publishMemberStateBatch(batches[key]); err != nil {
			return err
		}
	}

	return nil
}

func (g groupServiceImpl) publishMemberStateBatch(batch *memberStateBatch) error {
	if batch == nil || len(batch.states) == 0 {
		return nil
	}

	if len(batch.states) == 1 {
		return g.ep.GroupMemberStateChanged(singleMemberStateChangedPayload(batch, batch.states[0]))
	}

	return g.ep.GroupMemberStatesChanged(&events.GroupEventMemberStatesChangedPayload{
		ServiceID:           groupserver.ServiceID,
		RealmID:             batch.realmID,
		GroupID:             batch.groupID,
		SourceGatewayID:     batch.sourceGatewayID,
		SourceWorldserverID: batch.sourceWorldserverID,
		States:              batch.states,
		Receivers:           append([]uint64(nil), batch.receivers...),
	})
}

func (g *groupServiceImpl) publishMemberStateCatchup(realmID uint32, group *repo.Group, receiverGUID uint64) error {
	if group == nil || receiverGUID == 0 {
		return nil
	}

	groupRealmID := groupHomeRealmID(realmID, group)
	receiverGUID = groupPlayerGUID(groupRealmID, realmID, receiverGUID)
	states := g.stateTracker().memberStateUpdatesForGroup(groupRealmID, group.Members, receiverGUID)
	if len(states) == 0 {
		return nil
	}

	log.Debug().
		Uint32("realmID", groupRealmID).
		Uint("groupID", group.ID).
		Uint64("receiverGUID", receiverGUID).
		Int("stateCount", len(states)).
		Msg("publishing group member-state catch-up")

	for _, state := range states {
		if event := groupstatetrace.Event(nil, "groupserver.member_state.catchup", receiverGUID, state.MemberGUID); event != nil {
			event.
				Uint32("realmID", groupRealmID).
				Uint("groupID", group.ID).
				Uint64("receiverGUID", receiverGUID).
				Uint64("memberGUID", state.MemberGUID).
				Bool("online", state.Online).
				Uint8("level", state.Level).
				Uint8("class", state.Class).
				Uint32("zoneID", state.ZoneID).
				Uint32("mapID", state.MapID).
				Uint32("health", state.Health).
				Uint32("maxHealth", state.MaxHealth).
				Uint8("powerType", state.PowerType).
				Uint32("power", state.Power).
				Uint32("maxPower", state.MaxPower).
				Bool("aurasKnown", state.AurasKnown).
				Int("auraCount", len(state.Auras)).
				Str("auraSpells", formatEventMemberAuraTrace(state.Auras)).
				Msg(groupstatetrace.Message)
		}
	}

	return g.publishMemberStateBatch(&memberStateBatch{
		realmID:   groupRealmID,
		groupID:   group.ID,
		receivers: []uint64{receiverGUID},
		states:    states,
	})
}

func (g *groupServiceImpl) publishMemberStateCatchupsForOnlineMembers(realmID uint32, group *repo.Group) error {
	if group == nil {
		return nil
	}

	groupRealmID := groupHomeRealmID(realmID, group)
	for _, receiverGUID := range group.OnlineMemberGUIDs() {
		if err := g.publishMemberStateCatchup(groupRealmID, group, receiverGUID); err != nil {
			return err
		}
	}

	return nil
}

func singleMemberStateChangedPayload(batch *memberStateBatch, state events.GroupMemberStateUpdate) *events.GroupEventMemberStateChangedPayload {
	return &events.GroupEventMemberStateChangedPayload{
		ServiceID:           groupserver.ServiceID,
		RealmID:             batch.realmID,
		GroupID:             batch.groupID,
		SourceGatewayID:     batch.sourceGatewayID,
		SourceWorldserverID: batch.sourceWorldserverID,
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
		DeadKnown:           state.DeadKnown,
		Dead:                state.Dead,
		GhostKnown:          state.GhostKnown,
		Ghost:               state.Ghost,
		Receivers:           append([]uint64(nil), batch.receivers...),
	}
}

func memberStateReceiversKey(receivers []uint64) string {
	if len(receivers) == 0 {
		return ""
	}

	sortedReceivers := append([]uint64(nil), receivers...)
	sort.Slice(sortedReceivers, func(i, j int) bool {
		return sortedReceivers[i] < sortedReceivers[j]
	})

	parts := make([]string, 0, len(sortedReceivers))
	for _, receiver := range sortedReceivers {
		parts = append(parts, strconv.FormatUint(receiver, 10))
	}
	return strings.Join(parts, ",")
}

func traceMemberStateSnapshot(event *zerolog.Event, realmID uint32, snapshot MemberStateSnapshot) *zerolog.Event {
	event = event.
		Uint32("realmID", realmID).
		Uint64("memberGUID", snapshot.MemberGUID).
		Bool("online", snapshot.Online).
		Uint8("level", snapshot.Level).
		Uint8("class", snapshot.Class).
		Uint32("zoneID", snapshot.ZoneID).
		Uint32("mapID", snapshot.MapID).
		Bool("hasInstance", snapshot.InstanceID != nil).
		Uint32("health", snapshot.Health).
		Uint32("maxHealth", snapshot.MaxHealth).
		Uint8("powerType", snapshot.PowerType).
		Uint32("power", snapshot.Power).
		Uint32("maxPower", snapshot.MaxPower).
		Bool("hasDead", snapshot.Dead != nil).
		Bool("dead", boolPtrValue(snapshot.Dead)).
		Bool("hasGhost", snapshot.Ghost != nil).
		Bool("ghost", boolPtrValue(snapshot.Ghost)).
		Bool("aurasKnown", snapshot.AurasKnown).
		Int("auraCount", len(snapshot.Auras)).
		Str("auraSpells", formatMemberAuraTrace(snapshot.Auras)).
		Uint64("timestampMs", snapshot.TimestampMs)

	if snapshot.InstanceID != nil {
		event = event.Uint32("instanceID", *snapshot.InstanceID)
	}

	return event
}

func normalizeFixedClassMemberPower(snapshot MemberStateSnapshot) MemberStateSnapshot {
	if !wow.IsFixedClassInactivePowerType(snapshot.Class, snapshot.PowerType) {
		return snapshot
	}
	if snapshot.Power == 0 && snapshot.MaxPower == 0 {
		return snapshot
	}

	snapshot.PowerType, _ = wow.FixedPrimaryPowerTypeForClass(snapshot.Class)
	snapshot.Power = 0
	if snapshot.MaxPower == 0 {
		snapshot.MaxPower = wow.DefaultMaxPowerForClass(snapshot.Class)
	}

	return snapshot
}

func formatMemberAuraTrace(auras []MemberAuraState) string {
	auras = normalizeMemberAuras(auras)
	if len(auras) == 0 {
		return ""
	}

	parts := make([]string, 0, len(auras))
	for _, aura := range auras {
		parts = append(parts, strconv.Itoa(int(aura.Slot))+":"+strconv.FormatUint(uint64(aura.SpellID), 10)+":"+strconv.Itoa(int(aura.Flags)))
	}

	return strings.Join(parts, ",")
}

func formatEventMemberAuraTrace(auras []events.GroupMemberAuraState) string {
	if len(auras) == 0 {
		return ""
	}

	normalized := make([]events.GroupMemberAuraState, 0, len(auras))
	for _, aura := range auras {
		if aura.SpellID == 0 {
			continue
		}
		normalized = append(normalized, aura)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].Slot < normalized[j].Slot
	})

	parts := make([]string, 0, len(normalized))
	for _, aura := range normalized {
		parts = append(parts, strconv.Itoa(int(aura.Slot))+":"+strconv.FormatUint(uint64(aura.SpellID), 10)+":"+strconv.Itoa(int(aura.Flags)))
	}

	return strings.Join(parts, ",")
}

func normalizeMemberAuras(auras []MemberAuraState) []MemberAuraState {
	const maxGroupAuraSlots = 64

	if len(auras) == 0 {
		return nil
	}

	bySlot := make(map[uint8]MemberAuraState, len(auras))
	for _, aura := range auras {
		if aura.Slot >= maxGroupAuraSlots || aura.SpellID == 0 {
			continue
		}
		bySlot[aura.Slot] = aura
	}

	out := make([]MemberAuraState, 0, len(bySlot))
	for _, aura := range bySlot {
		out = append(out, aura)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Slot < out[j].Slot
	})

	return out
}

func memberAurasToEvent(auras []MemberAuraState) []events.GroupMemberAuraState {
	if len(auras) == 0 {
		return nil
	}

	out := make([]events.GroupMemberAuraState, 0, len(auras))
	for _, aura := range auras {
		out = append(out, events.GroupMemberAuraState{
			Slot:    aura.Slot,
			SpellID: aura.SpellID,
			Flags:   aura.Flags,
		})
	}

	return out
}

func (g groupServiceImpl) ResetInstance(ctx context.Context, realmID uint32, playerGUID uint64, mapID uint32, difficulty uint8) error {
	group, err := g.GroupByMemberGUID(ctx, realmID, playerGUID)
	if err != nil {
		return err
	}

	groupID := uint(0)
	receivers := []uint64{playerGUID}

	if group != nil {
		if group.LeaderGUID != playerGUID {
			return ErrNoPermissions
		}

		groupID = group.ID
		receivers = group.OnlineMemberGUIDs()
	}

	return g.ep.GroupInstanceResetRequest(&events.GroupEventInstanceResetRequestPayload{
		ServiceID:  groupserver.ServiceID,
		RealmID:    realmID,
		GroupID:    groupID,
		PlayerGUID: playerGUID,
		MapID:      mapID,
		Difficulty: difficulty,
		Receivers:  receivers,
	})
}

func (g groupServiceImpl) SetInstanceBindExtension(ctx context.Context, realmID uint32, playerGUID uint64, mapID uint32, difficulty uint8, extended bool) error {
	group, err := g.GroupByMemberGUID(ctx, realmID, playerGUID)
	if err != nil {
		return err
	}

	groupID := uint(0)
	if group != nil {
		groupID = group.ID
	}

	return g.ep.GroupInstanceBindExtensionRequest(&events.GroupEventInstanceBindExtensionRequestPayload{
		ServiceID:  groupserver.ServiceID,
		RealmID:    realmID,
		GroupID:    groupID,
		PlayerGUID: playerGUID,
		MapID:      mapID,
		Difficulty: difficulty,
		Extended:   extended,
		Receivers:  []uint64{playerGUID},
	})
}
