package session

import (
	"context"
	"fmt"
	"time"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	"github.com/walkline/ToCloud9/gen/group/pb"
	"github.com/walkline/ToCloud9/shared/events"
)

type GroupOperation uint8

const (
	GroupOperationInvite GroupOperation = iota
	GroupOperationUninvite
	GroupOperationLeave
	GroupOperationSwap
)

type GroupResult uint8

const (
	GroupResultOk GroupResult = iota
	GroupResultBadPlayerName
	GroupResultTargetNotInGroupS
	GroupResultTargetNotInInstanceS
	GroupResultGroupFull
	GroupResultAlreadyInGroup
	GroupResultNotInGroup
	GroupResultNotLeader
)

type groupResult struct {
	Operation  GroupOperation
	MemberName string
	Result     GroupResult
	SomeValue  uint32
}

func (r groupResult) BuildPacket() *packet.Packet {
	w := packet.NewWriterWithSize(packet.SMsgPartyCommandResult, uint32(4+len(r.MemberName)+1+4+4))
	w.Uint32(uint32(r.Operation))
	w.String(r.MemberName)
	w.Uint32(uint32(r.Result))
	w.Uint32(r.SomeValue)
	return w.ToPacket()
}

func (s *GameSession) HandleGroupInvite(ctx context.Context, p *packet.Packet) error {
	playerName := p.Reader().String()

	res := groupResult{
		Operation:  GroupOperationInvite,
		MemberName: playerName,
	}

	resp, err := s.charServiceClient.CharacterOnlineByName(ctx, &pbChar.CharacterOnlineByNameRequest{
		Api:           root.Ver,
		RealmID:       root.RealmID,
		CharacterName: playerName,
	})
	if err != nil {
		return err
	}

	if resp.Character == nil {
		res.Result = GroupResultBadPlayerName
		s.gameSocket.SendPacket(res.BuildPacket())
		return nil
	}

	inviteRes, err := s.groupServiceClient.Invite(ctx, &pb.InviteParams{
		Api:         root.SupportedGroupServiceVer,
		RealmID:     root.RealmID,
		Inviter:     s.character.GUID,
		Invited:     resp.Character.CharGUID,
		InviterName: s.character.Name,
		InvitedName: resp.Character.CharName,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	switch inviteRes.Status {
	case pb.InviteResponse_Ok:
		res.Result = GroupResultOk
	case pb.InviteResponse_AlreadyInGroup:
		res.Result = GroupResultAlreadyInGroup
	case pb.InviteResponse_GroupFull:
		res.Result = GroupResultGroupFull
	case pb.InviteResponse_NoPermissions:
		res.Result = GroupResultNotLeader
	default:
		return NewGroupServiceUnavailableErr(fmt.Errorf("unexpected invite status: %v", inviteRes.Status))
	}

	s.gameSocket.SendPacket(res.BuildPacket())

	return nil
}

func (s *GameSession) HandleEventGroupInviteCreated(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventInviteCreatedPayload)

	resp := packet.NewWriterWithSize(packet.SMsgGroupInvite, 0)
	resp.Uint8(1)
	resp.String(eventData.InviterName)
	resp.Uint32(0)
	resp.Uint8(0)
	resp.Uint32(0)
	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) HandleEventGroupMemberOnlineStatusChanged(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupMemberOnlineStatusChangedPayload)

	if !eventData.IsOnline {
		delete(s.groupMemberStats, eventData.MemberGUID)
	}

	s.publishCharacterStatsSnapshot()

	// TODO: we can handle this with less requests to the group service.
	return s.SendGroupUpdate(ctx, eventData.GroupID)
}

func (s *GameSession) HandleEventGroupCreated(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupCreatedPayload)

	var member *events.GroupMember
	for i, memberItr := range eventData.Members {
		if memberItr.MemberGUID == s.character.GUID {
			member = &eventData.Members[i]
			break
		}
	}

	if member == nil {
		return fmt.Errorf("group member not found for player %d", s.character.GUID)
	}

	s.publishCharacterStatsSnapshot()

	resp := packet.NewWriterWithSize(packet.SMsgGroupList, 0)
	resp.Uint8(eventData.GroupType)
	resp.Uint8(member.SubGroup)
	resp.Uint8(member.MemberFlags)
	resp.Uint8(member.Roles)

	resp.Uint64(member.MemberGUID)
	s.groupUpdateCounter++
	resp.Uint32(s.groupUpdateCounter)
	resp.Uint32(uint32(len(eventData.Members) - 1))
	for _, memberItr := range eventData.Members {
		if memberItr.MemberGUID == s.character.GUID {
			continue
		}

		var onlineFlag uint8 = 0
		if memberItr.IsOnline {
			onlineFlag = 1
		}

		resp.String(memberItr.MemberName)
		resp.Uint64(memberItr.MemberGUID)
		resp.Uint8(onlineFlag)
		resp.Uint8(memberItr.SubGroup)
		resp.Uint8(memberItr.MemberFlags)
		resp.Uint8(memberItr.Roles)
	}

	resp.Uint64(eventData.LeaderGUID)
	resp.Uint8(eventData.LootMethod)
	resp.Uint64(eventData.MasterLooterGuid)
	resp.Uint8(eventData.LootThreshold)
	resp.Uint8(eventData.Difficulty)
	resp.Uint8(eventData.RaidDifficulty)
	resp.Uint8(0) // heroic: m_raidDifficulty >= RAID_DIFFICULTY_10MAN_HEROIC

	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) HandleGroupInviteAccept(ctx context.Context, _ *packet.Packet) error {
	_, err := s.groupServiceClient.AcceptInvite(ctx, &pb.AcceptInviteParams{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	return nil
}

func (s *GameSession) HandleGroupInviteDeclined(ctx context.Context, _ *packet.Packet) error {

	return nil
}

func (s *GameSession) HandleGroupUninvite(ctx context.Context, p *packet.Packet) error {
	playerName := p.Reader().String()
	resp, err := s.charServiceClient.CharacterByName(ctx, &pbChar.CharacterByNameRequest{
		Api:           root.Ver,
		RealmID:       root.RealmID,
		CharacterName: playerName,
	})
	if err != nil {
		return err
	}

	res := groupResult{
		Operation:  GroupOperationUninvite,
		MemberName: playerName,
	}

	if resp.Character == nil {
		res.Result = GroupResultBadPlayerName
		s.gameSocket.SendPacket(res.BuildPacket())
		return nil
	}

	return s.groupUninviteWithGUID(ctx, resp.Character.CharGUID, resp.Character.CharName, "")
}

func (s *GameSession) HandleGroupUninviteGUID(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	guid := r.Uint64()
	reason := r.String()

	return s.groupUninviteWithGUID(ctx, guid, "", reason)
}

func (s *GameSession) HandleGroupLeave(ctx context.Context, _ *packet.Packet) error {
	_, err := s.groupServiceClient.Leave(ctx, &pb.GroupLeaveParams{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	res := groupResult{
		Operation: GroupOperationLeave,
		Result:    GroupResultOk,
	}
	s.gameSocket.SendPacket(res.BuildPacket())
	return nil
}

func (s *GameSession) HandleGroupConvertToRaid(ctx context.Context, _ *packet.Packet) error {
	_, err := s.groupServiceClient.ConvertToRaid(ctx, &pb.ConvertToRaidParams{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	res := groupResult{
		Operation: GroupOperationInvite,
		Result:    GroupResultOk,
	}
	s.gameSocket.SendPacket(res.BuildPacket())
	return nil
}

func (s *GameSession) HandleGroupSetLeader(ctx context.Context, p *packet.Packet) error {
	_, err := s.groupServiceClient.ChangeLeader(ctx, &pb.ChangeLeaderParams{
		Api:       root.SupportedGroupServiceVer,
		RealmID:   root.RealmID,
		Player:    s.character.GUID,
		NewLeader: p.Reader().Uint64(),
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	return nil
}

func (s *GameSession) HandleSetGroupTargetIcon(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	iconIDOrAction := reader.Uint8()
	const getListOfTargetIconsAction = 0xFF

	if iconIDOrAction == getListOfTargetIconsAction {
		return s.sendGroupListOfTargetIcons(ctx)
	}

	_, err := s.groupServiceClient.SetGroupTargetIcon(ctx, &pb.SetGroupTargetIconRequest{
		Api:        root.SupportedGroupServiceVer,
		RealmID:    root.RealmID,
		SetterGUID: s.character.GUID,
		IconID:     uint32(iconIDOrAction),
		TargetGUID: reader.Uint64(),
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	return nil
}

func (s *GameSession) HandleSetLootMethod(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()

	method := reader.Uint32()
	looter := reader.Uint64()
	threshold := reader.Uint32()

	_, err := s.groupServiceClient.SetLootMethod(ctx, &pb.SetLootMethodRequest{
		Api:           root.SupportedGroupServiceVer,
		RealmID:       root.RealmID,
		PlayerGUID:    s.character.GUID,
		Method:        method,
		LootMaster:    looter,
		LootThreshold: threshold,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	return nil
}

func (s *GameSession) HandleSetDungeonDifficulty(ctx context.Context, p *packet.Packet) error {
	if p.Source == packet.SourceWorldServer {
		s.worldSocket.SendPacket(p)
		return nil
	}

	difficulty := p.Reader().Uint32()

	groupResp, err := s.groupServiceClient.GetGroupByMember(ctx, &pb.GetGroupByMemberRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	if groupResp.Group == nil {
		s.worldSocket.SendPacket(p)
		return nil
	}

	if groupResp.Group.Difficulty == difficulty {
		return nil
	}

	_, err = s.groupServiceClient.SetDungeonDifficulty(ctx, &pb.SetDungeonDifficultyRequest{
		Api:        root.SupportedGroupServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		Difficulty: difficulty,
	})

	return err
}

func (s *GameSession) HandleSetRaidDifficulty(ctx context.Context, p *packet.Packet) error {
	if p.Source == packet.SourceWorldServer {
		s.worldSocket.SendPacket(p)
		return nil
	}

	difficulty := p.Reader().Uint32()

	groupResp, err := s.groupServiceClient.GetGroupByMember(ctx, &pb.GetGroupByMemberRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	if groupResp.Group == nil {
		s.worldSocket.SendPacket(p)
		return nil
	}

	if groupResp.Group.RaidDifficulty == difficulty {
		return nil
	}

	_, err = s.groupServiceClient.SetRaidDifficulty(ctx, &pb.SetRaidDifficultyRequest{
		Api:        root.SupportedGroupServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		Difficulty: difficulty,
	})

	return err
}

func (s *GameSession) HandleEventGroupNewTargetIcon(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventNewTargetIconPayload)

	const singleItemPacket = 0

	resp := packet.NewWriterWithSize(packet.MsgRaidTargetUpdate, uint32(1+8+1+8))
	resp.Uint8(singleItemPacket)
	resp.Uint64(eventData.Updater)
	resp.Uint8(eventData.IconID)
	resp.Uint64(eventData.Target)

	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) HandleEventGroupDifficultyChanged(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupDifficultyChangedPayload)

	if eventData.DungeonDifficulty != nil {
		resp := packet.NewWriterWithSize(packet.MsgSetDungeonDifficulty, 12)
		resp.Uint32(uint32(*eventData.DungeonDifficulty))
		resp.Uint32(1)
		resp.Uint32(1)
		s.gameSocket.Send(resp)
	}

	if eventData.RaidDifficulty != nil {
		resp := packet.NewWriterWithSize(packet.MsgSetRaidDifficulty, 12)
		resp.Uint32(uint32(*eventData.RaidDifficulty))
		resp.Uint32(1)
		resp.Uint32(1)
		s.gameSocket.Send(resp)
	}

	return nil
}

func (s *GameSession) sendGroupListOfTargetIcons(ctx context.Context) error {
	gr, err := s.groupServiceClient.GetGroupByMember(ctx, &pb.GetGroupByMemberRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	if gr.Group == nil {
		return nil
	}

	const isListOfTargets = uint8(1)

	resp := packet.NewWriter(packet.MsgRaidTargetUpdate)
	resp.Uint8(isListOfTargets)
	for i, targetGUID := range gr.Group.TargetIconsList {
		if targetGUID == 0 {
			continue
		}

		resp.Uint8(uint8(i))
		resp.Uint64(targetGUID)
	}

	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) groupUninviteWithGUID(ctx context.Context, player uint64, playerName, reason string) error {
	_, err := s.groupServiceClient.Uninvite(ctx, &pb.UninviteParams{
		Api:       root.SupportedGroupServiceVer,
		RealmID:   root.RealmID,
		Initiator: s.character.GUID,
		Target:    player,
		Reason:    reason,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	res := groupResult{
		Operation:  GroupOperationUninvite,
		MemberName: playerName,
		Result:     GroupResultOk,
	}
	s.gameSocket.SendPacket(res.BuildPacket())

	return nil
}

func (s *GameSession) LoadGroupForPlayer(ctx context.Context) error {
	res, err := s.groupServiceClient.GetGroupIDByPlayer(ctx, &pb.GetGroupIDByPlayerRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	if res.GroupID == 0 {
		return nil
	}

	return s.SendGroupUpdate(ctx, uint(res.GroupID))
}

func (s *GameSession) SendGroupUpdate(ctx context.Context, groupID uint) error {
	groupResp, err := s.groupServiceClient.GetGroup(ctx, &pb.GetGroupRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		GroupID: uint32(groupID),
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	if groupResp.Group == nil {
		return nil
	}

	var member *pb.GetGroupResponse_GroupMember
	for i, memberItr := range groupResp.Group.Members {
		if memberItr.Guid == s.character.GUID {
			member = groupResp.Group.Members[i]
			break
		}
	}

	if member == nil {
		return fmt.Errorf("group member not found for player %d", s.character.GUID)
	}

	resp := packet.NewWriterWithSize(packet.SMsgGroupList, 0)
	resp.Uint8(uint8(groupResp.Group.GroupType))
	resp.Uint8(uint8(member.SubGroup))
	resp.Uint8(uint8(member.Flags))
	resp.Uint8(uint8(member.Roles))

	resp.Uint64(uint64(groupResp.Group.Id))
	s.groupUpdateCounter++
	resp.Uint32(s.groupUpdateCounter)
	resp.Uint32(uint32(len(groupResp.Group.Members) - 1))
	for _, memberItr := range groupResp.Group.Members {
		if memberItr.Guid == s.character.GUID {
			continue
		}

		var onlineFlag uint8 = 0
		if memberItr.IsOnline {
			onlineFlag = 1
		}

		resp.String(memberItr.Name)
		resp.Uint64(memberItr.Guid)
		resp.Uint8(onlineFlag)
		resp.Uint8(uint8(memberItr.SubGroup))
		resp.Uint8(uint8(memberItr.Flags))
		resp.Uint8(uint8(memberItr.Roles))
	}

	resp.Uint64(groupResp.Group.Leader)
	resp.Uint8(uint8(groupResp.Group.LootMethod))
	resp.Uint64(groupResp.Group.MasterLooter)
	resp.Uint8(uint8(groupResp.Group.LootThreshold))
	resp.Uint8(uint8(groupResp.Group.Difficulty))
	resp.Uint8(uint8(groupResp.Group.RaidDifficulty))
	resp.Uint8(0) // heroic: m_raidDifficulty >= RAID_DIFFICULTY_10MAN_HEROIC

	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) HandleEventGroupMemberLeft(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupMemberLeftPayload)

	if eventData.MemberGUID == s.character.GUID {
		s.groupMemberStats = nil

		resp := packet.NewWriterWithSize(packet.SMsgGroupUnInvite, 0)
		s.gameSocket.Send(resp)

		s.groupUpdateCounter++

		resp = packet.NewWriterWithSize(packet.SMsgGroupList, 0)
		resp.Uint8(0x10).Uint8(0).Uint8(0).Uint8(0)
		resp.Uint64(uint64(eventData.GroupID)).Uint32(s.groupUpdateCounter).Uint32(0).Uint64(0)
		s.gameSocket.Send(resp)
		return nil
	}

	return s.SendGroupUpdate(ctx, eventData.GroupID)
}

func (s *GameSession) HandleEventGroupDisband(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupDisbandPayload)

	s.groupMemberStats = nil

	s.groupUpdateCounter++

	resp := packet.NewWriterWithSize(packet.SMsgGroupList, 0)
	resp.Uint8(0x10).Uint8(0).Uint8(0).Uint8(0)
	resp.Uint64(uint64(eventData.GroupID)).Uint32(s.groupUpdateCounter).Uint32(0).Uint64(0)
	s.gameSocket.Send(resp)

	resp = packet.NewWriterWithSize(packet.SMsgGroupDestroyed, 0)
	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) HandleEventGroupMemberAdded(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupMemberAddedPayload)

	s.publishCharacterStatsSnapshot()

	return s.SendGroupUpdate(ctx, eventData.GroupID)
}

func (s *GameSession) HandleEventGroupLeaderChanged(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupLeaderChangedPayload)
	return s.SendGroupUpdate(ctx, eventData.GroupID)
}

func (s *GameSession) HandleEventGroupLootTypeChanged(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupLootTypeChangedPayload)
	return s.SendGroupUpdate(ctx, eventData.GroupID)
}

func (s *GameSession) HandleEventGroupConvertedToRaid(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupConvertedToRaidPayload)
	return s.SendGroupUpdate(ctx, eventData.GroupID)
}

// Group update flags of the SMSG_PARTY_MEMBER_STATS packet (see AC Group.h GroupUpdateFlags).
const (
	groupUpdateFlagStatus    = 0x00000001 // uint16
	groupUpdateFlagCurHP     = 0x00000002 // uint32
	groupUpdateFlagMaxHP     = 0x00000004 // uint32
	groupUpdateFlagPowerType = 0x00000008 // uint8
	groupUpdateFlagCurPower  = 0x00000010 // uint16
	groupUpdateFlagMaxPower  = 0x00000020 // uint16
	groupUpdateFlagLevel     = 0x00000040 // uint16
	groupUpdateFlagZone      = 0x00000080 // uint16
)

// memberStatusOnline is the online bit of the member status flags (see AC Group.h GroupMemberOnlineStatus).
const memberStatusOnline = 0x0001

func (s *GameSession) HandleEventGroupMembersUpdated(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupMembersUpdatedPayload)

	for i := range eventData.Updates {
		upd := &eventData.Updates[i]
		if upd.MemberGUID == s.character.GUID {
			continue
		}

		s.storeGroupMemberStats(upd)
		s.gameSocket.SendPacket(buildPartyMemberStatsPacket(upd))
	}

	return nil
}

// HandleRequestPartyMemberStats answers stats requests for group members that the
// game server is not aware of (gateway-managed groups): without this interception the
// game server responds with an "offline" stub that marks the member as disconnected.
func (s *GameSession) HandleRequestPartyMemberStats(ctx context.Context, p *packet.Packet) error {
	if s.character == nil || s.character.GroupMangedByGameServer {
		s.worldSocket.SendPacket(p)
		return nil
	}

	guid := p.Reader().Uint64()

	stats, found := s.groupMemberStats[guid]
	if !found {
		// The cache starts empty (fresh login or post-redirect session rebuild)
		// and only ever receives changed fields, so a miss doesn't mean "not a
		// member". Confirm with the group service before falling through: the
		// game server would answer with an "offline" stub for a live member.
		// Offline members do fall through — for them the "offline" stub is the
		// right answer, while a fabricated status would show a disconnected
		// member as online with a full health bar. Pets and other non-player
		// GUIDs (high type bits set) fall through too.
		if guid>>48 == 0 && s.isOnlineInPlayersGroup(ctx, guid) {
			s.gameSocket.SendPacket(buildPartyMemberStatsPacket(&events.GroupMemberStatsUpdate{MemberGUID: guid}))
			return nil
		}

		s.worldSocket.SendPacket(p)
		return nil
	}

	// A FULL response resets every field its mask doesn't carry, and the cache
	// only holds fields that changed since it was created (a member whose HP
	// never changed has no HP there): an incomplete FULL zeroes health client
	// side, showing the member as dead. Answer incrementally unless every field
	// is known — the client then keeps its last known values.
	if groupMemberStatsComplete(&stats) {
		s.gameSocket.SendPacket(buildPartyMemberStatsFullPacket(guid, &stats))
	} else {
		s.gameSocket.SendPacket(buildPartyMemberStatsPacket(&stats))
	}

	return nil
}

func groupMemberStatsComplete(stats *events.GroupMemberStatsUpdate) bool {
	return stats.CurHP != nil && stats.MaxHP != nil && stats.PowerType != nil &&
		stats.CurPower != nil && stats.MaxPower != nil && stats.Level != nil && stats.Zone != nil
}

// groupMembersSnapshotTTL bounds how often a stats request for an unknown GUID
// may trigger a group service call.
const groupMembersSnapshotTTL = time.Second * 3

// isOnlineInPlayersGroup reports whether the given GUID belongs to the same group
// as the session's character and that member is currently online. The membership
// snapshot is memoized briefly so spamming requests with random GUIDs can't turn
// into one RPC per packet.
func (s *GameSession) isOnlineInPlayersGroup(ctx context.Context, guid uint64) bool {
	if s.groupServiceClient == nil {
		return false
	}

	if s.groupMembersSnapshot == nil || time.Since(s.groupMembersSnapshotAt) > groupMembersSnapshotTTL {
		gr, err := s.groupServiceClient.GetGroupByMember(ctx, &pb.GetGroupByMemberRequest{
			Api:     root.SupportedGroupServiceVer,
			RealmID: root.RealmID,
			Player:  s.character.GUID,
		})
		if err != nil {
			return false
		}

		snapshot := map[uint64]bool{}
		if gr.Group != nil {
			for _, member := range gr.Group.Members {
				snapshot[member.Guid] = member.IsOnline
			}
		}
		s.groupMembersSnapshot = snapshot
		s.groupMembersSnapshotAt = time.Now()
	}

	return s.groupMembersSnapshot[guid]
}

func (s *GameSession) storeGroupMemberStats(upd *events.GroupMemberStatsUpdate) {
	if s.groupMemberStats == nil {
		s.groupMemberStats = map[uint64]events.GroupMemberStatsUpdate{}
	}

	merged := s.groupMemberStats[upd.MemberGUID]
	merged.MemberGUID = upd.MemberGUID
	if upd.Level != nil {
		merged.Level = upd.Level
	}
	if upd.Zone != nil {
		merged.Zone = upd.Zone
	}
	if upd.CurHP != nil {
		merged.CurHP = upd.CurHP
	}
	if upd.MaxHP != nil {
		merged.MaxHP = upd.MaxHP
	}
	if upd.PowerType != nil {
		merged.PowerType = upd.PowerType
	}
	if upd.CurPower != nil {
		merged.CurPower = upd.CurPower
	}
	if upd.MaxPower != nil {
		merged.MaxPower = upd.MaxPower
	}
	s.groupMemberStats[upd.MemberGUID] = merged
}

// buildPartyMemberStatsFullPacket builds an SMSG_PARTY_MEMBER_STATS_FULL packet from
// the last known stats of a group member, as an answer to CMSG_REQUEST_PARTY_MEMBER_STATS.
func buildPartyMemberStatsFullPacket(guid uint64, stats *events.GroupMemberStatsUpdate) *packet.Packet {
	w := packet.NewWriter(packet.SMsgPartyMemberStatsFull)
	w.Uint8(0) // arena/bg related flag
	writePartyMemberStats(w, guid, stats)
	return w.ToPacket()
}

// buildPartyMemberStatsPacket builds an incremental SMSG_PARTY_MEMBER_STATS packet.
// The status field is always included so that members reported as offline by the
// game server (which is not aware of gateway-managed groups) get back online.
func buildPartyMemberStatsPacket(upd *events.GroupMemberStatsUpdate) *packet.Packet {
	w := packet.NewWriter(packet.SMsgPartyMemberStats)
	writePartyMemberStats(w, upd.MemberGUID, upd)
	return w.ToPacket()
}

func writePartyMemberStats(w *packet.Writer, guid uint64, upd *events.GroupMemberStatsUpdate) {
	mask := uint32(groupUpdateFlagStatus)

	if upd.CurHP != nil {
		mask |= groupUpdateFlagCurHP
	}
	if upd.MaxHP != nil {
		mask |= groupUpdateFlagMaxHP
	}
	if upd.PowerType != nil {
		mask |= groupUpdateFlagPowerType
	}
	if upd.CurPower != nil {
		mask |= groupUpdateFlagCurPower
	}
	if upd.MaxPower != nil {
		mask |= groupUpdateFlagMaxPower
	}
	if upd.Level != nil {
		mask |= groupUpdateFlagLevel
	}
	if upd.Zone != nil {
		mask |= groupUpdateFlagZone
	}

	w.GUID(guid)
	w.Uint32(mask)

	w.Uint16(memberStatusOnline)

	if upd.CurHP != nil {
		w.Uint32(*upd.CurHP)
	}
	if upd.MaxHP != nil {
		w.Uint32(*upd.MaxHP)
	}
	if upd.PowerType != nil {
		w.Uint8(*upd.PowerType)
	}
	if upd.CurPower != nil {
		w.Uint16(uint16(*upd.CurPower))
	}
	if upd.MaxPower != nil {
		w.Uint16(uint16(*upd.MaxPower))
	}
	if upd.Level != nil {
		w.Uint16(uint16(*upd.Level))
	}
	if upd.Zone != nil {
		w.Uint16(uint16(*upd.Zone))
	}
}

func (s *GameSession) HandleEventGroupNewMessage(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventNewMessagePayload)

	if eventData.SenderGUID == s.character.GUID {
		return nil
	}

	resp := packet.NewWriterWithSize(packet.SMsgMessageChat, 0)
	resp.Uint8(eventData.MessageType)
	resp.Uint32(eventData.Language)
	resp.Uint64(eventData.SenderGUID)
	resp.Uint32(0) // some flags
	resp.Uint64(eventData.SenderGUID)
	resp.Uint32(uint32(len(eventData.Msg) + 1))
	resp.String(eventData.Msg)
	resp.Uint8(0) // chat tag
	s.gameSocket.Send(resp)

	return nil
}
