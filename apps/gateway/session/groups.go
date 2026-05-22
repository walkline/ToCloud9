package session

import (
	"context"
	"fmt"
	"strings"

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

const (
	groupTypeFlagLFG             uint8 = 0x08
	groupLfgDungeonStateNotDone  uint8 = 0
	groupLfgDungeonStateFinished uint8 = 2
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

func (s *GameSession) characterNameRealm(ctx context.Context, characterName string) (uint32, string) {
	name, realmName, hasRealm := strings.Cut(characterName, "-")
	if !hasRealm || name == "" || realmName == "" || s.realmNamesService == nil {
		return root.RealmID, characterName
	}

	realmID, err := s.realmNamesService.IDByName(ctx, realmName)
	if err != nil {
		return root.RealmID, characterName
	}

	return realmID, name
}

func scopedCharacterGUID(requestRealmID uint32, characterRealmID uint32, characterGUID uint64) uint64 {
	if characterRealmID == 0 {
		characterRealmID = requestRealmID
	}

	return playerObjectGUIDForRealm(characterRealmID, characterGUID)
}

func (s *GameSession) HandleGroupInvite(ctx context.Context, p *packet.Packet) error {
	playerName := p.Reader().String()
	targetRealmID, targetName := s.characterNameRealm(ctx, playerName)

	res := groupResult{
		Operation:  GroupOperationInvite,
		MemberName: playerName,
	}

	resp, err := s.charServiceClient.CharacterOnlineByName(ctx, &pbChar.CharacterOnlineByNameRequest{
		Api:           root.Ver,
		RealmID:       targetRealmID,
		CharacterName: targetName,
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
		Invited:     scopedCharacterGUID(targetRealmID, resp.Character.RealmID, resp.Character.CharGUID),
		InviterName: s.character.Name,
		InvitedName: resp.Character.CharName,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	if inviteRes.Status != pb.InviteResponse_Ok {
		res.Result = 16
	} else {
		res.Result = GroupResultOk
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

func (s *GameSession) HandleEventGroupInviteDeclined(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventInviteDeclinedPayload)

	resp := packet.NewWriterWithSize(packet.SMsgGroupDecline, uint32(len(eventData.InviteeName)+1))
	resp.String(eventData.InviteeName)
	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) HandleEventGroupMemberOnlineStatusChanged(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupMemberOnlineStatusChangedPayload)
	if s.character == nil || samePlayerGUID(eventData.RealmID, eventData.MemberGUID, sessionRealmID(eventData.RealmID), s.character.GUID) {
		return nil
	}
	if s.clusterGroupPresentationBlocked() {
		return nil
	}

	return s.SendGroupUpdateInRealm(ctx, eventData.RealmID, eventData.GroupID)
}

func (s *GameSession) HandleEventGroupCreated(ctx context.Context, e *eBroadcaster.Event) error {
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
	_, err := s.groupServiceClient.DeclineInvite(ctx, &pb.DeclineInviteParams{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	return nil
}

func (s *GameSession) HandleGroupUninvite(ctx context.Context, p *packet.Packet) error {
	if forwarded, err := s.forwardGroupUninviteToWorldserverIfLfgDungeon(ctx, p); forwarded || err != nil {
		return err
	}

	playerName := p.Reader().String()

	groupResp, err := s.groupServiceClient.GetGroupByMember(ctx, &pb.GetGroupByMemberRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	if groupResp.Group != nil {
		if member := s.groupMemberByName(ctx, groupResp.Group, playerName); member != nil {
			return s.groupUninviteWithGUID(ctx, member.Guid, member.Name, "")
		}
	}

	targetRealmID, targetName := s.characterNameRealm(ctx, playerName)
	resp, err := s.charServiceClient.CharacterByName(ctx, &pbChar.CharacterByNameRequest{
		Api:           root.Ver,
		RealmID:       targetRealmID,
		CharacterName: targetName,
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

	return s.groupUninviteWithGUID(ctx, scopedCharacterGUID(targetRealmID, resp.Character.RealmID, resp.Character.CharGUID), resp.Character.CharName, "")
}

func (s *GameSession) HandleGroupUninviteGUID(ctx context.Context, p *packet.Packet) error {
	if forwarded, err := s.forwardGroupUninviteToWorldserverIfLfgDungeon(ctx, p); forwarded || err != nil {
		return err
	}

	r := p.Reader()
	guid := playerDBGUIDFromClientGUID(r.Uint64())
	reason := r.String()

	return s.groupUninviteWithGUID(ctx, guid, "", reason)
}

func (s *GameSession) forwardGroupUninviteToWorldserverIfLfgDungeon(ctx context.Context, p *packet.Packet) (bool, error) {
	return s.forwardPacketToWorldserverIfLfgDungeon(ctx, p)
}

func (s *GameSession) HandleGroupLeave(ctx context.Context, p *packet.Packet) error {
	forwarded, err := s.forwardPacketToWorldserverIfLfgDungeon(ctx, p)
	if err != nil {
		return err
	}

	if s.groupServiceClient == nil {
		if forwarded {
			return nil
		}
		return NewGroupServiceUnavailableErr(fmt.Errorf("group service client is nil"))
	}

	_, err = s.groupServiceClient.Leave(ctx, &pb.GroupLeaveParams{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		if forwarded {
			if s.logger != nil {
				s.logger.Warn().Err(err).Msg("forwarded LFG dungeon group leave to worldserver but groupservice cleanup failed")
			}
			return nil
		}
		return NewGroupServiceUnavailableErr(err)
	}

	if forwarded {
		return nil
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
		if isSilentGroupMutationError(err) {
			return nil
		}
		return NewGroupServiceUnavailableErr(err)
	}

	res := groupResult{
		Operation: GroupOperationInvite,
		Result:    GroupResultOk,
	}
	s.gameSocket.SendPacket(res.BuildPacket())
	return s.SendCurrentGroupUpdate(ctx)
}

func (s *GameSession) HandleGroupSetLeader(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	newLeader := playerDBGUIDFromClientGUID(r.Uint64())
	if err := r.Error(); err != nil {
		return err
	}

	_, err := s.groupServiceClient.ChangeLeader(ctx, &pb.ChangeLeaderParams{
		Api:       root.SupportedGroupServiceVer,
		RealmID:   root.RealmID,
		Player:    s.character.GUID,
		NewLeader: newLeader,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	return s.SendCurrentGroupUpdate(ctx)
}

func (s *GameSession) HandleSetGroupTargetIcon(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	iconIDOrAction := reader.Uint8()
	const getListOfTargetIconsAction = 0xFF

	if iconIDOrAction == getListOfTargetIconsAction {
		return s.sendGroupListOfTargetIcons(ctx)
	}

	targetGUID := playerDBGUIDFromClientGUID(reader.Uint64())
	if err := reader.Error(); err != nil {
		return err
	}

	_, err := s.groupServiceClient.SetGroupTargetIcon(ctx, &pb.SetGroupTargetIconRequest{
		Api:        root.SupportedGroupServiceVer,
		RealmID:    root.RealmID,
		SetterGUID: s.character.GUID,
		IconID:     uint32(iconIDOrAction),
		TargetGUID: targetGUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	return nil
}

func (s *GameSession) HandleSetLootMethod(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()

	method := reader.Uint32()
	looter := playerDBGUIDFromClientGUID(reader.Uint64())
	threshold := reader.Uint32()
	if err := reader.Error(); err != nil {
		return err
	}

	_, err := s.groupServiceClient.SetLootMethod(ctx, &pb.SetLootMethodRequest{
		Api:           root.SupportedGroupServiceVer,
		RealmID:       root.RealmID,
		PlayerGUID:    s.character.GUID,
		Method:        method,
		LootMaster:    looter,
		LootThreshold: threshold,
	})
	if err != nil {
		if isSilentGroupMutationError(err) {
			return nil
		}
		return NewGroupServiceUnavailableErr(err)
	}

	return nil
}

func (s *GameSession) HandleSetDungeonDifficulty(ctx context.Context, p *packet.Packet) error {
	if p.Source == packet.SourceWorldServer {
		s.worldSocket.SendPacket(p)
		return nil
	}

	reader := p.Reader()
	difficulty := reader.Uint32()
	if err := reader.Error(); err != nil {
		return err
	}

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

	resp, err := s.groupServiceClient.SetDungeonDifficulty(ctx, &pb.SetDungeonDifficultyRequest{
		Api:        root.SupportedGroupServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		Difficulty: difficulty,
	})
	if err != nil {
		if isSilentGroupMutationError(err) {
			return nil
		}
		return NewGroupServiceUnavailableErr(err)
	}
	if resp.GetStatus() != pb.SetDungeonDifficultyResponse_Ok {
		s.sendDungeonDifficulty(groupResp.Group.Difficulty)
	}

	return nil
}

func (s *GameSession) HandleSetRaidDifficulty(ctx context.Context, p *packet.Packet) error {
	if p.Source == packet.SourceWorldServer {
		s.worldSocket.SendPacket(p)
		return nil
	}

	reader := p.Reader()
	difficulty := reader.Uint32()
	if err := reader.Error(); err != nil {
		return err
	}

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

	resp, err := s.groupServiceClient.SetRaidDifficulty(ctx, &pb.SetRaidDifficultyRequest{
		Api:        root.SupportedGroupServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		Difficulty: difficulty,
	})
	if err != nil {
		if isSilentGroupMutationError(err) {
			return nil
		}
		return NewGroupServiceUnavailableErr(err)
	}
	if resp.GetStatus() != pb.SetRaidDifficultyResponse_Ok {
		s.sendRaidDifficulty(groupResp.Group.RaidDifficulty)
	}

	return nil
}

func (s *GameSession) HandleEventGroupNewTargetIcon(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventNewTargetIconPayload)

	const singleItemPacket = 0

	resp := packet.NewWriterWithSize(packet.MsgRaidTargetUpdate, uint32(1+8+1+8))
	resp.Uint8(singleItemPacket)
	resp.Uint64(playerObjectGUIDForRealm(eventData.RealmID, eventData.Updater))
	resp.Uint8(eventData.IconID)
	resp.Uint64(playerObjectGUIDForRealm(eventData.RealmID, eventData.Target))

	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) HandleEventGroupDifficultyChanged(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupDifficultyChangedPayload)

	if eventData.DungeonDifficulty != nil {
		s.sendDungeonDifficulty(uint32(*eventData.DungeonDifficulty))
	}

	if eventData.RaidDifficulty != nil {
		s.sendRaidDifficulty(uint32(*eventData.RaidDifficulty))
	}

	return nil
}

func (s *GameSession) sendDungeonDifficulty(difficulty uint32) {
	resp := packet.NewWriterWithSize(packet.MsgSetDungeonDifficulty, 12)
	resp.Uint32(difficulty)
	resp.Uint32(1)
	resp.Uint32(1)
	s.gameSocket.Send(resp)
}

func (s *GameSession) sendRaidDifficulty(difficulty uint32) {
	resp := packet.NewWriterWithSize(packet.MsgSetRaidDifficulty, 12)
	resp.Uint32(difficulty)
	resp.Uint32(1)
	resp.Uint32(1)
	s.gameSocket.Send(resp)
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
		groupRealmID := groupHomeRealmIDFromPB(gr.Group)
		resp.Uint64(playerObjectGUIDForRealm(groupRealmID, targetGUID))
	}

	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) groupUninviteWithGUID(ctx context.Context, player uint64, playerName, reason string) error {
	player = playerDBGUIDFromClientGUID(player)

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

	groupRealmID := res.GroupRealmID
	if groupRealmID == 0 {
		groupRealmID = root.RealmID
	}
	return s.SendGroupUpdateInRealm(ctx, groupRealmID, uint(res.GroupID))
}

func (s *GameSession) SendGroupUpdate(ctx context.Context, groupID uint) error {
	return s.SendGroupUpdateInRealm(ctx, root.RealmID, groupID)
}

func (s *GameSession) SendGroupUpdateInRealm(ctx context.Context, realmID uint32, groupID uint) error {
	groupResp, err := s.groupServiceClient.GetGroup(ctx, &pb.GetGroupRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: realmID,
		GroupID: uint32(groupID),
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	if groupResp.Group == nil {
		return nil
	}

	return s.sendGroupUpdateFromGroup(groupResp.Group)
}

func (s *GameSession) SendCurrentGroupUpdate(ctx context.Context) error {
	if s.character == nil {
		return nil
	}

	groupResp, err := s.groupServiceClient.GetGroupByMember(ctx, &pb.GetGroupByMemberRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	if groupResp.Group == nil {
		return nil
	}

	return s.sendGroupUpdateFromGroup(groupResp.Group)
}

func (s *GameSession) sendGroupUpdateFromGroup(group *pb.GetGroupResponse_Group) error {
	if s.clusterGroupPresentationBlocked() {
		return nil
	}

	groupRealmID := groupHomeRealmIDFromPB(group)
	var member *pb.GetGroupResponse_GroupMember
	for i, memberItr := range group.Members {
		if samePlayerGUID(groupRealmID, memberItr.Guid, sessionRealmID(groupRealmID), s.character.GUID) {
			member = group.Members[i]
			break
		}
	}

	if member == nil {
		return fmt.Errorf("group member not found for player %d", s.character.GUID)
	}

	resp := packet.NewWriterWithSize(packet.SMsgGroupList, 0)
	resp.Uint8(uint8(group.GroupType))
	resp.Uint8(uint8(member.SubGroup))
	resp.Uint8(uint8(member.Flags))
	resp.Uint8(uint8(member.Roles))
	if uint8(group.GroupType)&groupTypeFlagLFG != 0 {
		s.writeGroupListLfgFields(resp)
	}

	resp.Uint64(groupObjectGUID(uint64(group.Id)))
	s.groupUpdateCounter++
	resp.Uint32(s.groupUpdateCounter)
	resp.Uint32(uint32(len(group.Members) - 1))
	for _, memberItr := range group.Members {
		if samePlayerGUID(groupRealmID, memberItr.Guid, sessionRealmID(groupRealmID), s.character.GUID) {
			continue
		}

		var onlineFlag uint8 = 0
		if memberItr.IsOnline {
			onlineFlag = 1
		}

		resp.String(memberItr.Name)
		resp.Uint64(playerObjectGUIDForMember(groupRealmID, memberItr))
		resp.Uint8(onlineFlag)
		resp.Uint8(uint8(memberItr.SubGroup))
		resp.Uint8(uint8(memberItr.Flags))
		resp.Uint8(uint8(memberItr.Roles))
	}

	resp.Uint64(playerObjectGUIDForRealm(groupRealmID, group.Leader))
	resp.Uint8(uint8(group.LootMethod))
	resp.Uint64(playerObjectGUIDForRealm(groupRealmID, group.MasterLooter))
	resp.Uint8(uint8(group.LootThreshold))
	resp.Uint8(uint8(group.Difficulty))
	resp.Uint8(uint8(group.RaidDifficulty))
	resp.Uint8(0) // heroic: m_raidDifficulty >= RAID_DIFFICULTY_10MAN_HEROIC

	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) writeGroupListLfgFields(resp *packet.Writer) {
	state := groupLfgDungeonStateNotDone
	dungeon := uint32(0)
	if s.character != nil {
		status := s.character.lastLfgStatus
		if status.State == events.MatchmakingLfgStateFinishedDungeon {
			state = groupLfgDungeonStateFinished
		}
		dungeon = lfgProposalDisplayDungeon(status)
	}

	resp.Uint8(state)
	resp.Uint32(dungeon)
}

func (s *GameSession) HandleEventGroupMemberLeft(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupMemberLeftPayload)
	if s.character == nil {
		return nil
	}
	if samePlayerGUID(eventData.RealmID, eventData.MemberGUID, sessionRealmID(eventData.RealmID), s.character.GUID) {
		s.gameSocket.Send(packet.NewWriterWithSize(packet.SMsgGroupUnInvite, 0))
		return nil
	}

	if s.lfgDungeonActive && !s.clusterGroupPresentationBlocked() && groupMemberLeftEventIncludesSession(eventData, s.character.GUID) {
		return s.SendGroupUpdateInRealm(ctx, eventData.RealmID, eventData.GroupID)
	}

	// AzerothCore applies membership updates from libsidecar and emits the client roster.
	return nil
}

func groupMemberLeftEventIncludesSession(eventData *events.GroupEventGroupMemberLeftPayload, playerGUID uint64) bool {
	if eventData == nil || playerGUID == 0 {
		return false
	}

	for _, memberGUID := range eventData.OnlineMembers {
		if samePlayerGUID(eventData.RealmID, memberGUID, sessionRealmID(eventData.RealmID), playerGUID) {
			return true
		}
	}

	return false
}

func (s *GameSession) HandleEventGroupDisband(ctx context.Context, e *eBroadcaster.Event) error {
	if s.clusterGroupPresentationBlocked() {
		return nil
	}

	s.gameSocket.Send(packet.NewWriterWithSize(packet.SMsgGroupDestroyed, 0))

	// AzerothCore applies disband updates from libsidecar and emits the client roster.
	return nil
}

func (s *GameSession) clusterGroupPresentationBlocked() bool {
	return s != nil &&
		s.character != nil &&
		(s.pendingMapTransferRouting != nil ||
			s.activeMapTransferRouting != nil ||
			s.teleportingToNewMap != nil ||
			s.pendingRedirectID != "")
}

func (s *GameSession) HandleEventGroupMemberAdded(ctx context.Context, e *eBroadcaster.Event) error {
	if s.character == nil {
		return nil
	}

	// AzerothCore applies this event through libsidecar and emits the roster.
	return nil
}

func (s *GameSession) HandleEventGroupLeaderChanged(ctx context.Context, e *eBroadcaster.Event) error {
	if s.character == nil {
		return nil
	}

	// AzerothCore applies this event through libsidecar and emits both leader and roster packets.
	return nil
}

func (s *GameSession) HandleEventGroupLootTypeChanged(ctx context.Context, e *eBroadcaster.Event) error {
	if s.character == nil {
		return nil
	}

	// AzerothCore applies this event through libsidecar and emits the roster.
	return nil
}

func (s *GameSession) HandleEventGroupConvertedToRaid(ctx context.Context, e *eBroadcaster.Event) error {
	if s.character == nil {
		return nil
	}

	// AzerothCore applies this event through libsidecar and emits the roster.
	return nil
}

func (s *GameSession) sendGroupLeaderChanged(ctx context.Context, groupID uint, newLeaderGUID uint64) error {
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
	groupRealmID := groupHomeRealmIDFromPB(groupResp.Group)

	for _, member := range groupResp.Group.Members {
		if !samePlayerGUID(groupRealmID, member.Guid, groupRealmID, newLeaderGUID) {
			continue
		}

		resp := packet.NewWriterWithSize(packet.SMsgGroupSetLeader, uint32(len(member.Name)+1))
		resp.String(member.Name)
		s.gameSocket.Send(resp)
		return nil
	}

	return nil
}

func (s *GameSession) HandleEventGroupNewMessage(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventNewMessagePayload)

	if samePlayerGUID(eventData.RealmID, eventData.SenderGUID, sessionRealmID(eventData.RealmID), s.character.GUID) {
		return nil
	}

	s.sendAzerothCorePlayerChat(ChatType(eventData.MessageType), eventData.Language, eventData.RealmID, eventData.SenderGUID, eventData.SenderName, 0, "", eventData.Msg, eventData.SenderChatTag)

	return nil
}
