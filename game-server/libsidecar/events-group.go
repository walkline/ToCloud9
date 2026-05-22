package main

/*
#include "events-group.h"
*/
import "C"

import (
	"context"
	"fmt"
	zlog "github.com/rs/zerolog/log"
	"time"
	"unsafe"

	charserver "github.com/walkline/ToCloud9/apps/charserver"
	groupserver "github.com/walkline/ToCloud9/apps/groupserver"
	charPb "github.com/walkline/ToCloud9/gen/characters/pb"
	groupPb "github.com/walkline/ToCloud9/gen/group/pb"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/game-server/libsidecar/consumer"
	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/groupstatetrace"
	wowguid "github.com/walkline/ToCloud9/shared/wow/guid"
)

// TC9SetOnGroupCreatedHook sets hook for group created event.
//
//export TC9SetOnGroupCreatedHook
func TC9SetOnGroupCreatedHook(h C.OnGroupCreatedHook) {
	C.SetOnGroupCreatedHook(h)
}

// TC9SetOnGroupMemberAddedHook sets hook for member added event.
//
//export TC9SetOnGroupMemberAddedHook
func TC9SetOnGroupMemberAddedHook(h C.OnGroupMemberAddedHook) {
	C.SetOnGroupMemberAddedHook(h)
}

// TC9SetOnGroupMemberRemovedHook sets hook for member left/kicked event.
//
//export TC9SetOnGroupMemberRemovedHook
func TC9SetOnGroupMemberRemovedHook(h C.OnGroupMemberRemovedHook) {
	C.SetOnGroupMemberRemovedHook(h)
}

// TC9SetOnGroupLeaderChangedHook sets hook for leader changed event.
//
//export TC9SetOnGroupLeaderChangedHook
func TC9SetOnGroupLeaderChangedHook(h C.OnGroupLeaderChangedHook) {
	C.SetOnGroupLeaderChangedHook(h)
}

// TC9SetOnGroupDisbandedHook sets hook for group disbanded event.
//
//export TC9SetOnGroupDisbandedHook
func TC9SetOnGroupDisbandedHook(h C.OnGroupDisbandedHook) {
	C.SetOnGroupDisbandedHook(h)
}

// TC9SetOnGroupLootTypeChangedHook sets hook for group loot type changed event.
//
//export TC9SetOnGroupLootTypeChangedHook
func TC9SetOnGroupLootTypeChangedHook(h C.OnGroupLootTypeChangedHook) {
	C.SetOnGroupLootTypeChangedHook(h)
}

// TC9SetOnGroupDungeonDifficultyChangedHook sets hook for group dungeon difficulty changed event.
//
//export TC9SetOnGroupDungeonDifficultyChangedHook
func TC9SetOnGroupDungeonDifficultyChangedHook(h C.OnGroupDungeonDifficultyChangedHook) {
	C.SetOnGroupDungeonDifficultyChangedHook(h)
}

// TC9SetOnGroupRaidDifficultyChangedHook sets hook for group raid difficulty changed event.
//
//export TC9SetOnGroupRaidDifficultyChangedHook
func TC9SetOnGroupRaidDifficultyChangedHook(h C.OnGroupRaidDifficultyChangedHook) {
	C.SetOnGroupRaidDifficultyChangedHook(h)
}

// TC9SetOnGroupConvertedToRaidHook sets hook for group converted to raid event.
//
//export TC9SetOnGroupConvertedToRaidHook
func TC9SetOnGroupConvertedToRaidHook(h C.OnGroupConvertedToRaidHook) {
	C.SetOnGroupConvertedToRaidHook(h)
}

//export TC9SetOnGroupReadyCheckStartedHook
func TC9SetOnGroupReadyCheckStartedHook(h C.OnGroupReadyCheckStartedHook) {
	C.SetOnGroupReadyCheckStartedHook(h)
}

//export TC9SetOnGroupReadyCheckMemberStateHook
func TC9SetOnGroupReadyCheckMemberStateHook(h C.OnGroupReadyCheckMemberStateHook) {
	C.SetOnGroupReadyCheckMemberStateHook(h)
}

//export TC9SetOnGroupReadyCheckFinishedHook
func TC9SetOnGroupReadyCheckFinishedHook(h C.OnGroupReadyCheckFinishedHook) {
	C.SetOnGroupReadyCheckFinishedHook(h)
}

//export TC9SetOnGroupMemberSubGroupChangedHook
func TC9SetOnGroupMemberSubGroupChangedHook(h C.OnGroupMemberSubGroupChangedHook) {
	C.SetOnGroupMemberSubGroupChangedHook(h)
}

//export TC9SetOnGroupMemberFlagsChangedHook
func TC9SetOnGroupMemberFlagsChangedHook(h C.OnGroupMemberFlagsChangedHook) {
	C.SetOnGroupMemberFlagsChangedHook(h)
}

//export TC9SetOnGroupMemberStateChangedHook
func TC9SetOnGroupMemberStateChangedHook(h C.OnGroupMemberStateChangedHook) {
	C.SetOnGroupMemberStateChangedHook(h)
}

//export TC9SetOnGroupInstanceResetRequestHook
func TC9SetOnGroupInstanceResetRequestHook(h C.OnGroupInstanceResetRequestHook) {
	C.SetOnGroupInstanceResetRequestHook(h)
}

//export TC9SetOnGroupInstanceBindExtensionRequestHook
func TC9SetOnGroupInstanceBindExtensionRequestHook(h C.OnGroupInstanceBindExtensionRequestHook) {
	C.SetOnGroupInstanceBindExtensionRequestHook(h)
}

type groupHandlerFabric struct {
	logger zerolog.Logger
}

func playerObjectGUIDForRealm(realmID uint32, guid uint64) C.uint64_t {
	// Libsidecar sends AzerothCore-facing player ObjectGuid values. Local
	// members are low DB GUIDs; foreign members are realm-scoped player GUIDs.
	playerRealmID := wowguid.PlayerRealmIDOrDefault(realmID, guid)
	return C.uint64_t(wowguid.PlayerGUIDForRealm(RealmID, playerRealmID, guid))
}

func playerDBGUIDFromObjectGUID(guid C.uint64_t) uint64 {
	raw := uint64(guid)
	if raw == 0 || raw>>32 == 0 || raw>>48 != 0 {
		return raw
	}

	if uint32((raw>>32)&0xffff) != RealmID {
		return raw
	}

	return raw & 0xffffffff
}

func playerServiceKeyFromObjectGUID(defaultRealmID uint32, guid uint64) (uint32, uint64) {
	raw := guid
	return wowguid.PlayerRealmIDOrDefault(defaultRealmID, raw), wowguid.PlayerLowGUID(raw)
}

//export TC9RegisterMaterializedLfgGroup
func TC9RegisterMaterializedLfgGroup(
	groupGuid C.uint32_t,
	leaderGuid C.uint64_t,
	groupType C.uint8_t,
	difficulty C.uint8_t,
	raidDifficulty C.uint8_t,
	members *C.GroupMaterializedLfgMember,
	membersSize C.uint8_t,
) {
	if groupServiceClient == nil || groupGuid == 0 || leaderGuid == 0 || members == nil || membersSize == 0 {
		return
	}

	groupRealmID, leaderLowGUID := playerServiceKeyFromObjectGUID(RealmID, uint64(leaderGuid))
	if groupRealmID == 0 {
		zlog.Error().
			Uint32("realmID", groupRealmID).
			Uint64("leaderRaw", uint64(leaderGuid)).
			Uint32("groupID", uint32(groupGuid)).
			Msg("failed to register materialized LFG group with unknown group realm")
		return
	}

	cMembers := unsafe.Slice(members, int(membersSize))
	requestMembers := make([]*groupPb.MaterializedLfgGroupMember, 0, len(cMembers))
	for _, member := range cMembers {
		if member.memberGuid == 0 {
			continue
		}

		memberRealmID, memberLowGUID := playerServiceKeyFromObjectGUID(groupRealmID, uint64(member.memberGuid))
		memberName := ""
		if member.memberName != nil {
			memberName = C.GoString(member.memberName)
		}

		requestMembers = append(requestMembers, &groupPb.MaterializedLfgGroupMember{
			RealmID:    memberRealmID,
			PlayerGUID: memberLowGUID,
			Name:       memberName,
			IsOnline:   member.online != 0,
			Flags:      uint32(member.flags),
			Roles:      uint32(member.roles),
			SubGroup:   uint32(member.subGroup),
		})
	}

	if len(requestMembers) == 0 {
		return
	}

	if event := groupstatetrace.Event(nil, "libsidecar.register_materialized_lfg_group.rpc", leaderLowGUID); event != nil {
		event.
			Uint32("realmID", groupRealmID).
			Uint32("groupID", uint32(groupGuid)).
			Uint64("leaderGUID", leaderLowGUID).
			Uint64("leaderRaw", uint64(leaderGuid)).
			Uint8("groupType", uint8(groupType)).
			Int("memberCount", len(requestMembers)).
			Msg(groupstatetrace.Message)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := groupServiceClient.RegisterMaterializedLfgGroup(ctx, &groupPb.RegisterMaterializedLfgGroupRequest{
		Api:            groupserver.Ver,
		RealmID:        groupRealmID,
		GroupID:        uint32(groupGuid),
		LeaderGUID:     leaderLowGUID,
		GroupType:      uint32(groupType),
		Difficulty:     uint32(difficulty),
		RaidDifficulty: uint32(raidDifficulty),
		Members:        requestMembers,
	})
	if err != nil {
		zlog.Error().
			Err(err).
			Uint32("realmID", groupRealmID).
			Uint32("groupID", uint32(groupGuid)).
			Uint64("leaderGUID", leaderLowGUID).
			Int("memberCount", len(requestMembers)).
			Msg("failed to register materialized LFG group")
	}
}

func NewGroupHandlerFabric(logger zerolog.Logger) consumer.GroupHandlersFabric {
	return &groupHandlerFabric{
		logger: logger,
	}
}

func (g groupHandlerFabric) GroupCreated(payload *events.GroupEventGroupCreatedPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		cMemberGUIDs, cMemberNames, membersSize, releaseMembers := groupMembersToC(payload.RealmID, payload.Members)
		defer releaseMembers()

		var group C.EventObjectGroup
		group.guid = C.uint32_t(payload.GroupID)
		group.leader = playerObjectGUIDForRealm(payload.RealmID, payload.LeaderGUID)
		group.lootMethod = C.uint8_t(payload.LootMethod)
		group.looterGuid = playerObjectGUIDForRealm(payload.RealmID, payload.LooterGUID)
		group.lootThreshold = C.uint8_t(payload.LootThreshold)
		group.groupType = C.uint8_t(payload.GroupType)
		group.difficulty = C.uint8_t(payload.Difficulty)
		group.raidDifficulty = C.uint8_t(payload.RaidDifficulty)
		group.masterLooterGuid = playerObjectGUIDForRealm(payload.RealmID, payload.MasterLooterGuid)
		group.members = cMemberGUIDs
		group.memberNames = cMemberNames
		group.membersSize = membersSize
		group.lfgDungeonEntry = C.uint32_t(payload.LfgDungeonEntry)

		r := C.CallOnGroupCreatedHook(&group)
		g.handleResponse(int(r), "GroupCreated")
	})
}

func groupMembersToC(realmID uint32, members []events.GroupMember) (*C.uint64_t, **C.char, C.uint8_t, func()) {
	release := func() {}
	if len(members) == 0 {
		return nil, nil, 0, release
	}

	guidPtr := C.malloc(C.size_t(len(members)) * C.size_t(unsafe.Sizeof(C.uint64_t(0))))
	if guidPtr == nil {
		return nil, nil, 0, release
	}

	namePtr := C.malloc(C.size_t(len(members)) * C.size_t(unsafe.Sizeof(uintptr(0))))
	if namePtr == nil {
		C.free(guidPtr)
		return nil, nil, 0, release
	}

	cMemberGUIDs := unsafe.Slice((*C.uint64_t)(guidPtr), len(members))
	cMemberNames := unsafe.Slice((**C.char)(namePtr), len(members))
	for i, member := range members {
		cMemberGUIDs[i] = playerObjectGUIDForRealm(realmID, member.MemberGUID)
		cMemberNames[i] = C.CString(member.MemberName)
	}

	release = func() {
		for _, name := range cMemberNames {
			if name != nil {
				C.free(unsafe.Pointer(name))
			}
		}
		C.free(namePtr)
		C.free(guidPtr)
	}

	return (*C.uint64_t)(guidPtr), (**C.char)(namePtr), C.uint8_t(len(members)), release
}

func (g groupHandlerFabric) GroupMemberAdded(payload *events.GroupEventGroupMemberAddedPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		r := C.CallOnGroupMemberAddedHook(C.uint32_t(payload.GroupID), playerObjectGUIDForRealm(payload.RealmID, payload.MemberGUID))
		g.handleResponse(int(r), "GroupMemberAdded")
	})
}

func (g groupHandlerFabric) GroupMemberRemoved(payload *events.GroupEventGroupMemberLeftPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		r := C.CallOnGroupMemberRemovedHook(
			C.uint32_t(payload.GroupID),
			playerObjectGUIDForRealm(payload.RealmID, payload.MemberGUID),
			playerObjectGUIDForRealm(payload.RealmID, payload.NewLeaderID),
		)
		g.handleResponse(int(r), "GroupMemberRemoved")
	})
}

func (g groupHandlerFabric) GroupLeaderChanged(payload *events.GroupEventGroupLeaderChangedPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		r := C.CallOnGroupLeaderChangedHook(
			C.uint32_t(payload.GroupID),
			playerObjectGUIDForRealm(payload.RealmID, payload.PreviousLeader),
			playerObjectGUIDForRealm(payload.RealmID, payload.NewLeader),
		)
		g.handleResponse(int(r), "GroupLeaderChanged")
	})
}

func (g groupHandlerFabric) GroupDisbanded(payload *events.GroupEventGroupDisbandPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		r := C.CallOnGroupDisbandedHook(C.uint32_t(payload.GroupID))
		g.handleResponse(int(r), "GroupDisbanded")
	})
}

//export TC9UpdateGroupMemberState
func TC9UpdateGroupMemberState(
	memberGuid C.uint64_t,
	online C.uint8_t,
	level C.uint8_t,
	playerClass C.uint8_t,
	zoneId C.uint32_t,
	mapId C.uint32_t,
	health C.uint32_t,
	maxHealth C.uint32_t,
	powerType C.uint8_t,
	power C.uint32_t,
	maxPower C.uint32_t,
	instanceID C.uint32_t,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	memberRealmID, memberDBGuid := playerServiceKeyFromObjectGUID(RealmID, uint64(memberGuid))
	instanceIDValue := uint32(instanceID)
	if event := groupstatetrace.Event(nil, "libsidecar.update_member_state.rpc", memberDBGuid); event != nil {
		event.
			Uint32("realmID", memberRealmID).
			Uint64("memberGUID", memberDBGuid).
			Uint64("memberRaw", uint64(memberGuid)).
			Str("sourceWorldserverID", AssignedGameServerID).
			Bool("online", online != 0).
			Uint8("level", uint8(level)).
			Uint8("class", uint8(playerClass)).
			Uint32("zoneID", uint32(zoneId)).
			Uint32("mapID", uint32(mapId)).
			Uint32("health", uint32(health)).
			Uint32("maxHealth", uint32(maxHealth)).
			Uint8("powerType", uint8(powerType)).
			Uint32("power", uint32(power)).
			Uint32("maxPower", uint32(maxPower)).
			Uint32("instanceID", uint32(instanceID)).
			Msg(groupstatetrace.Message)
	}

	var instanceIDPtr *uint32
	if instanceIDValue != 0 {
		instanceIDPtr = &instanceIDValue
	}

	_, err := groupServiceClient.BulkUpdateMemberStates(ctx, &groupPb.BulkUpdateMemberStatesRequest{
		Api:                 groupserver.Ver,
		RealmID:             memberRealmID,
		SourceWorldserverID: AssignedGameServerID,
		Snapshots: []*groupPb.PlayerStateSnapshot{
			{
				MemberGUID:  memberDBGuid,
				Online:      online != 0,
				Level:       uint32(level),
				ClassID:     uint32(playerClass),
				ZoneID:      uint32(zoneId),
				MapID:       uint32(mapId),
				Health:      uint32(health),
				MaxHealth:   uint32(maxHealth),
				PowerType:   uint32(powerType),
				Power:       uint32(power),
				MaxPower:    uint32(maxPower),
				InstanceID:  instanceIDPtr,
				TimestampMs: uint64(time.Now().UnixMilli()),
			},
		},
	})
	if err != nil {
		zlog.Error().
			Err(err).
			Uint32("realmID", memberRealmID).
			Uint64("member", memberDBGuid).
			Uint64("memberRaw", uint64(memberGuid)).
			Bool("online", online != 0).
			Uint8("level", uint8(level)).
			Uint8("class", uint8(playerClass)).
			Uint32("zone", uint32(zoneId)).
			Uint32("health", uint32(health)).
			Uint32("maxHealth", uint32(maxHealth)).
			Uint8("powerType", uint8(powerType)).
			Uint32("power", uint32(power)).
			Uint32("maxPower", uint32(maxPower)).
			Uint32("instanceID", uint32(instanceID)).
			Msg("failed to update group member state")
	}
}

//export TC9StartGroupReadyCheck
func TC9StartGroupReadyCheck(leaderGuid C.uint64_t, durationMs C.uint32_t) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	leaderDBGuid := playerDBGUIDFromObjectGUID(leaderGuid)

	_, err := groupServiceClient.StartReadyCheck(ctx, &groupPb.StartReadyCheckRequest{
		Api:        groupserver.Ver,
		RealmID:    RealmID,
		LeaderGUID: leaderDBGuid,
		DurationMs: uint32(durationMs),
	})
	if err != nil {
		zlog.Error().
			Err(err).
			Uint64("leader", leaderDBGuid).
			Uint64("leaderRaw", uint64(leaderGuid)).
			Uint32("durationMs", uint32(durationMs)).
			Msg("failed to start group ready check")
	}
}

//export TC9ConfirmLfgDungeonRouteEntered
func TC9ConfirmLfgDungeonRouteEntered(
	memberGuid C.uint64_t,
	mapId C.uint32_t,
	difficulty C.uint8_t,
	instanceID C.uint32_t,
) {
	if characterServiceClient == nil || mapId == 0 || instanceID == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	memberRealmID, memberDBGuid := playerServiceKeyFromObjectGUID(RealmID, uint64(memberGuid))
	if event := groupstatetrace.Event(nil, "libsidecar.confirm_lfg_dungeon_route.rpc", memberDBGuid); event != nil {
		event.
			Uint32("realmID", memberRealmID).
			Uint64("memberGUID", memberDBGuid).
			Uint64("memberRaw", uint64(memberGuid)).
			Uint32("mapID", uint32(mapId)).
			Uint8("difficulty", uint8(difficulty)).
			Uint32("instanceID", uint32(instanceID)).
			Msg(groupstatetrace.Message)
	}

	_, err := characterServiceClient.ConfirmLfgDungeonRouteEntered(ctx, &charPb.ConfirmLfgDungeonRouteEnteredRequest{
		Api:        charserver.Ver,
		RealmID:    memberRealmID,
		PlayerGUID: memberDBGuid,
		MapID:      uint32(mapId),
		Difficulty: uint32(difficulty),
		InstanceID: uint32(instanceID),
	})
	if err != nil {
		zlog.Error().
			Err(err).
			Uint32("realmID", memberRealmID).
			Uint64("member", memberDBGuid).
			Uint64("memberRaw", uint64(memberGuid)).
			Uint32("map", uint32(mapId)).
			Uint8("difficulty", uint8(difficulty)).
			Uint32("instanceID", uint32(instanceID)).
			Msg("can't confirm LFG dungeon route")
	}
}

//export TC9SetGroupReadyCheckMemberState
func TC9SetGroupReadyCheckMemberState(memberGuid C.uint64_t, state C.uint8_t) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	memberDBGuid := playerDBGUIDFromObjectGUID(memberGuid)

	_, err := groupServiceClient.SetReadyCheckMemberState(ctx, &groupPb.SetReadyCheckMemberStateRequest{
		Api:        groupserver.Ver,
		RealmID:    RealmID,
		MemberGUID: memberDBGuid,
		State:      uint32(state),
	})
	if err != nil {
		zlog.Error().
			Err(err).
			Uint64("member", memberDBGuid).
			Uint64("memberRaw", uint64(memberGuid)).
			Uint8("state", uint8(state)).
			Msg("failed to set group ready check member state")
	}
}

//export TC9FinishGroupReadyCheck
func TC9FinishGroupReadyCheck(playerGuid C.uint64_t) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	playerDBGuid := playerDBGUIDFromObjectGUID(playerGuid)

	_, err := groupServiceClient.FinishReadyCheck(ctx, &groupPb.FinishReadyCheckRequest{
		Api:        groupserver.Ver,
		RealmID:    RealmID,
		PlayerGUID: playerDBGuid,
	})
	if err != nil {
		zlog.Error().
			Err(err).
			Uint64("player", playerDBGuid).
			Uint64("playerRaw", uint64(playerGuid)).
			Msg("failed to finish group ready check")
	}
}

//export TC9ChangeGroupMemberSubGroup
func TC9ChangeGroupMemberSubGroup(updaterGuid C.uint64_t, memberGuid C.uint64_t, subGroup C.uint8_t) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updaterDBGuid := playerDBGUIDFromObjectGUID(updaterGuid)
	memberDBGuid := playerDBGUIDFromObjectGUID(memberGuid)

	_, err := groupServiceClient.ChangeMemberSubGroup(ctx, &groupPb.ChangeMemberSubGroupRequest{
		Api:         groupserver.Ver,
		RealmID:     RealmID,
		UpdaterGUID: updaterDBGuid,
		MemberGUID:  memberDBGuid,
		SubGroup:    uint32(subGroup),
	})
	if err != nil {
		zlog.Error().
			Err(err).
			Uint64("updater", updaterDBGuid).
			Uint64("updaterRaw", uint64(updaterGuid)).
			Uint64("member", memberDBGuid).
			Uint64("memberRaw", uint64(memberGuid)).
			Uint8("subGroup", uint8(subGroup)).
			Msg("failed to change group member subgroup")
	}
}

//export TC9SetGroupMemberFlags
func TC9SetGroupMemberFlags(updaterGuid C.uint64_t, memberGuid C.uint64_t, flags C.uint8_t, roles C.uint8_t) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updaterDBGuid := playerDBGUIDFromObjectGUID(updaterGuid)
	memberDBGuid := playerDBGUIDFromObjectGUID(memberGuid)

	_, err := groupServiceClient.SetMemberFlags(ctx, &groupPb.SetMemberFlagsRequest{
		Api:         groupserver.Ver,
		RealmID:     RealmID,
		UpdaterGUID: updaterDBGuid,
		MemberGUID:  memberDBGuid,
		Flags:       uint32(flags),
		Roles:       uint32(roles),
	})
	if err != nil {
		zlog.Error().
			Err(err).
			Uint64("updater", updaterDBGuid).
			Uint64("updaterRaw", uint64(updaterGuid)).
			Uint64("member", memberDBGuid).
			Uint64("memberRaw", uint64(memberGuid)).
			Uint8("flags", uint8(flags)).
			Uint8("roles", uint8(roles)).
			Msg("failed to set group member flags")
	}
}

func (g groupHandlerFabric) GroupLootTypeChanged(payload *events.GroupEventGroupLootTypeChangedPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		r := C.CallOnGroupLootTypeChangedHook(
			C.uint32_t(payload.GroupID),
			C.uint8_t(payload.NewLootType),
			playerObjectGUIDForRealm(payload.RealmID, payload.NewLooterGUID),
			C.uint8_t(payload.NewLooterThreshold),
		)
		g.handleResponse(int(r), "LootTypeChanged")
	})
}

func (g groupHandlerFabric) GroupDifficultyChanged(payload *events.GroupEventGroupDifficultyChangedPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		fmt.Printf("DungeonDifficulty: %p, RaidDifficulty: %p\n", payload.DungeonDifficulty, payload.RaidDifficulty)
		if payload.DungeonDifficulty != nil {
			r := C.CallOnGroupDungeonDifficultyChangedHook(
				C.uint32_t(payload.GroupID),
				C.uint8_t(*payload.DungeonDifficulty),
			)
			g.handleResponse(int(r), "DungeonDifficultyChanged")
		}

		if payload.RaidDifficulty != nil {
			r := C.CallOnGroupRaidDifficultyChangedHook(
				C.uint32_t(payload.GroupID),
				C.uint8_t(*payload.RaidDifficulty),
			)
			g.handleResponse(int(r), "RaidDifficultyChanged")
		}
	})
}

func (g groupHandlerFabric) GroupConvertedToRaid(payload *events.GroupEventGroupConvertedToRaidPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		r := C.CallOnGroupConvertedToRaidHook(
			C.uint32_t(payload.GroupID),
		)
		g.handleResponse(int(r), "GroupConvertedToRaid")
	})
}

func (g groupHandlerFabric) handleResponse(resp int, hookName string) {
	const (
		CallStatusOk     = 0
		CallStatusNoHook = 1
	)
	switch resp {
	case CallStatusOk:
	case CallStatusNoHook:
		g.logger.Warn().Str("hook", hookName).Msg("no bound hook")
	default:
		g.logger.Error().Str("hook", hookName).Msg("unk status")
	}
}

func (g groupHandlerFabric) GroupReadyCheckStarted(payload *events.GroupEventReadyCheckStartedPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		req := C.GroupReadyCheckStarted{
			groupGuid:  C.uint32_t(payload.GroupID),
			leaderGuid: playerObjectGUIDForRealm(payload.RealmID, payload.LeaderGUID),
			durationMs: C.uint32_t(payload.DurationMs),
		}

		r := C.CallOnGroupReadyCheckStartedHook(&req)
		g.handleResponse(int(r), "GroupReadyCheckStarted")
	})
}

func (g groupHandlerFabric) GroupReadyCheckMemberState(payload *events.GroupEventReadyCheckMemberStatePayload) queue.Handler {
	return eventsHandlerFunc(func() {
		req := C.GroupReadyCheckMemberState{
			groupGuid:  C.uint32_t(payload.GroupID),
			memberGuid: playerObjectGUIDForRealm(payload.RealmID, payload.MemberGUID),
			state:      C.uint8_t(payload.State),
		}

		r := C.CallOnGroupReadyCheckMemberStateHook(&req)
		g.handleResponse(int(r), "GroupReadyCheckMemberState")
	})
}

func (g groupHandlerFabric) GroupReadyCheckFinished(payload *events.GroupEventReadyCheckFinishedPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		req := C.GroupReadyCheckFinished{
			groupGuid: C.uint32_t(payload.GroupID),
		}

		r := C.CallOnGroupReadyCheckFinishedHook(&req)
		g.handleResponse(int(r), "GroupReadyCheckFinished")
	})
}

func (g groupHandlerFabric) GroupMemberSubGroupChanged(payload *events.GroupEventMemberSubGroupChangedPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		req := C.GroupMemberSubGroupChanged{
			groupGuid:  C.uint32_t(payload.GroupID),
			memberGuid: playerObjectGUIDForRealm(payload.RealmID, payload.MemberGUID),
			subGroup:   C.uint8_t(payload.SubGroup),
		}

		r := C.CallOnGroupMemberSubGroupChangedHook(&req)
		g.handleResponse(int(r), "GroupMemberSubGroupChanged")
	})
}

func (g groupHandlerFabric) GroupMemberFlagsChanged(payload *events.GroupEventMemberFlagsChangedPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		req := C.GroupMemberFlagsChanged{
			groupGuid:  C.uint32_t(payload.GroupID),
			memberGuid: playerObjectGUIDForRealm(payload.RealmID, payload.MemberGUID),
			flags:      C.uint8_t(payload.Flags),
			roles:      C.uint8_t(payload.Roles),
		}

		r := C.CallOnGroupMemberFlagsChangedHook(&req)
		g.handleResponse(int(r), "GroupMemberFlagsChanged")
	})
}

func (g groupHandlerFabric) GroupMemberStateChanged(payload *events.GroupEventMemberStateChangedPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		if event := groupstatetrace.Event(nil, "libsidecar.group_member_state.hook", payload.MemberGUID); event != nil {
			event.
				Uint32("realmID", payload.RealmID).
				Uint("groupID", payload.GroupID).
				Uint64("memberGUID", payload.MemberGUID).
				Str("sourceGatewayID", payload.SourceGatewayID).
				Str("sourceWorldserverID", payload.SourceWorldserverID).
				Bool("online", payload.Online).
				Uint8("level", payload.Level).
				Uint8("class", payload.Class).
				Uint32("zoneID", payload.ZoneID).
				Uint32("mapID", payload.MapID).
				Uint32("health", payload.Health).
				Uint32("maxHealth", payload.MaxHealth).
				Uint8("powerType", payload.PowerType).
				Uint32("power", payload.Power).
				Uint32("maxPower", payload.MaxPower).
				Bool("aurasKnown", payload.AurasKnown).
				Int("auraCount", len(payload.Auras)).
				Msg(groupstatetrace.Message)
		}

		auras, auraCount, releaseAuras := groupMemberAurasToC(payload.AurasKnown, payload.Auras)
		defer releaseAuras()

		req := C.GroupMemberStateChanged{
			groupGuid:   C.uint32_t(payload.GroupID),
			memberGuid:  playerObjectGUIDForRealm(payload.RealmID, payload.MemberGUID),
			online:      C.uint8_t(boolToUint8(payload.Online)),
			level:       C.uint8_t(payload.Level),
			playerClass: C.uint8_t(payload.Class),
			zoneId:      C.uint32_t(payload.ZoneID),
			mapId:       C.uint32_t(payload.MapID),
			health:      C.uint32_t(payload.Health),
			maxHealth:   C.uint32_t(payload.MaxHealth),
			powerType:   C.uint8_t(payload.PowerType),
			power:       C.uint32_t(payload.Power),
			maxPower:    C.uint32_t(payload.MaxPower),
			aurasKnown:  C.uint8_t(boolToUint8(payload.AurasKnown)),
			auraCount:   auraCount,
			auras:       auras,
		}

		r := C.CallOnGroupMemberStateChangedHook(&req)
		g.handleResponse(int(r), "GroupMemberStateChanged")
	})
}

func groupMemberAurasToC(known bool, payloadAuras []events.GroupMemberAuraState) (*C.GroupMemberAuraState, C.uint32_t, func()) {
	if !known || len(payloadAuras) == 0 {
		return nil, 0, func() {}
	}

	ptr := C.malloc(C.size_t(len(payloadAuras)) * C.size_t(unsafe.Sizeof(C.GroupMemberAuraState{})))
	if ptr == nil {
		return nil, 0, func() {}
	}

	auras := unsafe.Slice((*C.GroupMemberAuraState)(ptr), len(payloadAuras))
	count := 0
	for _, aura := range payloadAuras {
		if aura.Slot >= 64 || aura.SpellID == 0 {
			continue
		}
		auras[count] = C.GroupMemberAuraState{
			slot:    C.uint8_t(aura.Slot),
			spellId: C.uint32_t(aura.SpellID),
			flags:   C.uint8_t(aura.Flags),
		}
		count++
	}

	if count == 0 {
		C.free(ptr)
		return nil, 0, func() {}
	}

	return (*C.GroupMemberAuraState)(ptr), C.uint32_t(count), func() {
		C.free(ptr)
	}
}

func (g groupHandlerFabric) GroupInstanceResetRequest(payload *events.GroupEventInstanceResetRequestPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		req := C.GroupInstanceResetRequest{
			groupGuid:  C.uint32_t(payload.GroupID),
			playerGuid: playerObjectGUIDForRealm(payload.RealmID, payload.PlayerGUID),
			mapId:      C.uint32_t(payload.MapID),
			difficulty: C.uint8_t(payload.Difficulty),
		}

		r := C.CallOnGroupInstanceResetRequestHook(&req)
		g.handleResponse(int(r), "GroupInstanceResetRequest")
	})
}

func (g groupHandlerFabric) GroupInstanceBindExtensionRequest(payload *events.GroupEventInstanceBindExtensionRequestPayload) queue.Handler {
	return eventsHandlerFunc(func() {
		req := C.GroupInstanceBindExtensionRequest{
			groupGuid:  C.uint32_t(payload.GroupID),
			playerGuid: playerObjectGUIDForRealm(payload.RealmID, payload.PlayerGUID),
			mapId:      C.uint32_t(payload.MapID),
			difficulty: C.uint8_t(payload.Difficulty),
			extended:   C.uint8_t(boolToUint8(payload.Extended)),
		}

		r := C.CallOnGroupInstanceBindExtensionRequestHook(&req)
		g.handleResponse(int(r), "GroupInstanceBindExtensionRequest")
	})
}

func boolToUint8(v bool) uint8 {
	if v {
		return 1
	}

	return 0
}
