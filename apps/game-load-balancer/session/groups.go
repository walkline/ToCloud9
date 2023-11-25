package session

import (
	"context"
	"fmt"

	root "github.com/walkline/ToCloud9/apps/game-load-balancer"
	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
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

func (s *GameSession) HandleEventGroupMemberOnlineStatusChanged(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventGroupMemberOnlineStatusChangedPayload)

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
