package server

import (
	"context"
	"errors"

	"github.com/walkline/ToCloud9/apps/groupserver"
	"github.com/walkline/ToCloud9/apps/groupserver/repo"
	"github.com/walkline/ToCloud9/apps/groupserver/service"
	"github.com/walkline/ToCloud9/gen/group/pb"
)

type GroupServer struct {
	pb.UnimplementedGroupServiceServer
	groupService service.GroupsService
}

func NewGroupServer(s service.GroupsService) pb.GroupServiceServer {
	return &GroupServer{
		groupService: s,
	}
}

func (g GroupServer) GetGroup(ctx context.Context, request *pb.GetGroupRequest) (*pb.GetGroupResponse, error) {
	group, err := g.groupService.GroupByID(ctx, request.RealmID, uint(request.GroupID))
	if err != nil {
		return nil, err
	}

	if group == nil {
		return &pb.GetGroupResponse{
			Api:   groupserver.Ver,
			Group: nil,
		}, nil
	}

	return g.getGroupResponse(group), nil
}

func (g GroupServer) GetGroupByMember(ctx context.Context, request *pb.GetGroupByMemberRequest) (*pb.GetGroupResponse, error) {
	group, err := g.groupService.GroupByMemberGUID(ctx, request.RealmID, request.Player)
	if err != nil {
		return nil, err
	}

	if group == nil {
		return &pb.GetGroupResponse{
			Api:   groupserver.Ver,
			Group: nil,
		}, nil
	}

	return g.getGroupResponse(group), nil
}

func (g GroupServer) GetGroupIDByPlayer(ctx context.Context, request *pb.GetGroupIDByPlayerRequest) (*pb.GetGroupIDByPlayerResponse, error) {
	groupRealmID, groupID, err := g.groupService.GroupRealmIDByPlayer(ctx, request.RealmID, request.Player)
	if err != nil {
		return nil, err
	}

	return &pb.GetGroupIDByPlayerResponse{
		Api:          groupserver.Ver,
		GroupID:      uint32(groupID),
		GroupRealmID: groupRealmID,
	}, nil
}

func (g GroupServer) GetMemberPlacements(ctx context.Context, request *pb.GetMemberPlacementsRequest) (*pb.GetMemberPlacementsResponse, error) {
	placements, err := g.groupService.MemberPlacements(ctx, request.RealmID, request.MemberGUIDs)
	if err != nil {
		return nil, err
	}

	res := &pb.GetMemberPlacementsResponse{
		Api:        groupserver.Ver,
		Placements: make([]*pb.MemberPlacement, 0, len(placements)),
	}
	for _, placement := range placements {
		res.Placements = append(res.Placements, &pb.MemberPlacement{
			MemberGUID:    placement.MemberGUID,
			Online:        placement.Online,
			Fresh:         placement.Fresh,
			GatewayID:     placement.GatewayID,
			WorldserverID: placement.WorldserverID,
			MapID:         placement.MapID,
			InstanceID:    placement.InstanceID,
			InstanceKnown: placement.InstanceKnown,
			TimestampMs:   placement.TimestampMs,
			UpdatedAtMs:   placement.UpdatedAtMs,
		})
	}

	return res, nil
}

func (g GroupServer) Invite(ctx context.Context, params *pb.InviteParams) (*pb.InviteResponse, error) {
	err := g.groupService.Invite(ctx, params.RealmID, params.Inviter, params.Invited, params.InviterName, params.InvitedName)
	if err != nil {
		return nil, err
	}

	status := pb.InviteResponse_Ok

	return &pb.InviteResponse{
		Api:    groupserver.Ver,
		Status: status,
	}, nil
}

func (g GroupServer) AcceptInvite(ctx context.Context, params *pb.AcceptInviteParams) (*pb.AcceptInviteResponse, error) {
	status := pb.AcceptInviteResponse_Ok
	err := g.groupService.AcceptInvite(ctx, params.RealmID, params.Player)
	if err != nil {
		if errors.Is(err, service.ErrInviteNotFound) {
			status = pb.AcceptInviteResponse_InviteNotFound
		} else {
			return nil, err
		}
	}

	return &pb.AcceptInviteResponse{
		Api:    groupserver.Ver,
		Status: status,
	}, nil
}

func (g GroupServer) DeclineInvite(ctx context.Context, params *pb.DeclineInviteParams) (*pb.DeclineInviteResponse, error) {
	status := pb.DeclineInviteResponse_Ok
	err := g.groupService.DeclineInvite(ctx, params.RealmID, params.Player)
	if err != nil {
		if errors.Is(err, service.ErrInviteNotFound) {
			status = pb.DeclineInviteResponse_InviteNotFound
		} else {
			return nil, err
		}
	}

	return &pb.DeclineInviteResponse{
		Api:    groupserver.Ver,
		Status: status,
	}, nil
}

func (g GroupServer) Uninvite(ctx context.Context, params *pb.UninviteParams) (*pb.UninviteResponse, error) {
	err := g.groupService.Uninvite(ctx, params.RealmID, params.Initiator, params.Target, params.Reason)
	if err != nil {
		return nil, err
	}

	status := pb.UninviteResponse_Ok

	return &pb.UninviteResponse{
		Api:    groupserver.Ver,
		Status: status,
	}, nil
}

func (g GroupServer) Leave(ctx context.Context, params *pb.GroupLeaveParams) (*pb.GroupLeaveResponse, error) {
	err := g.groupService.Leave(ctx, params.RealmID, params.Player)
	if err != nil {
		return nil, err
	}

	return &pb.GroupLeaveResponse{
		Api: groupserver.Ver,
	}, nil
}

func (g GroupServer) ConvertToRaid(ctx context.Context, params *pb.ConvertToRaidParams) (*pb.ConvertToRaidResponse, error) {
	err := g.groupService.ConvertToRaid(ctx, params.RealmID, params.Player)
	if err != nil {
		return nil, err
	}

	return &pb.ConvertToRaidResponse{
		Api: groupserver.Ver,
	}, nil
}

func (g GroupServer) ChangeLeader(ctx context.Context, params *pb.ChangeLeaderParams) (*pb.ChangeLeaderResponse, error) {
	err := g.groupService.ChangeLeader(ctx, params.RealmID, params.Player, params.NewLeader)
	if err != nil {
		return nil, err
	}

	return &pb.ChangeLeaderResponse{
		Api: groupserver.Ver,
	}, nil
}

func (g GroupServer) SendMessage(ctx context.Context, params *pb.SendGroupMessageParams) (*pb.SendGroupMessageResponse, error) {
	err := g.groupService.SendMessage(
		ctx,
		params.RealmID,
		params.SenderGUID,
		params.Message,
		params.Language,
		service.MessageType(params.MessageType),
		uint8(params.SenderChatTag),
	)
	if err != nil {
		return nil, err
	}

	return &pb.SendGroupMessageResponse{
		Api: groupserver.Ver,
	}, nil
}

func (g GroupServer) SetGroupTargetIcon(ctx context.Context, params *pb.SetGroupTargetIconRequest) (*pb.SetGroupTargetIconResponse, error) {
	err := g.groupService.SetTargetIcon(ctx, params.RealmID, params.SetterGUID, uint8(params.IconID), params.TargetGUID)
	if err != nil {
		return nil, err
	}

	return &pb.SetGroupTargetIconResponse{
		Api: groupserver.Ver,
	}, nil
}

func (g GroupServer) SetLootMethod(ctx context.Context, params *pb.SetLootMethodRequest) (*pb.SetLootMethodResponse, error) {
	err := g.groupService.SetLootMethod(ctx, params.RealmID, params.PlayerGUID, uint8(params.Method), params.LootMaster, uint8(params.LootThreshold))
	if err != nil {
		return nil, err
	}

	return &pb.SetLootMethodResponse{
		Api: groupserver.Ver,
	}, nil
}

func (g GroupServer) SetDungeonDifficulty(ctx context.Context, params *pb.SetDungeonDifficultyRequest) (*pb.SetDungeonDifficultyResponse, error) {
	err := g.groupService.SetDungeonDifficulty(ctx, params.RealmID, params.PlayerGUID, uint8(params.Difficulty))
	if err != nil {
		if errors.Is(err, service.ErrMemberInDungeonOrRaid) {
			return &pb.SetDungeonDifficultyResponse{
				Api:    groupserver.Ver,
				Status: pb.SetDungeonDifficultyResponse_MemberIsInDungeon,
			}, nil
		}
		return nil, err
	}

	return &pb.SetDungeonDifficultyResponse{
		Api:    groupserver.Ver,
		Status: pb.SetDungeonDifficultyResponse_Ok,
	}, nil
}

func (g GroupServer) SetRaidDifficulty(ctx context.Context, params *pb.SetRaidDifficultyRequest) (*pb.SetRaidDifficultyResponse, error) {
	err := g.groupService.SetRaidDifficulty(ctx, params.RealmID, params.PlayerGUID, uint8(params.Difficulty))
	if err != nil {
		if errors.Is(err, service.ErrMemberInDungeonOrRaid) {
			return &pb.SetRaidDifficultyResponse{
				Api:    groupserver.Ver,
				Status: pb.SetRaidDifficultyResponse_MemberIsInRaid,
			}, nil
		}
		return nil, err
	}

	return &pb.SetRaidDifficultyResponse{
		Api:    groupserver.Ver,
		Status: pb.SetRaidDifficultyResponse_Ok,
	}, nil
}

func (g GroupServer) getGroupResponse(group *repo.Group) *pb.GetGroupResponse {
	members := make([]*pb.GetGroupResponse_GroupMember, len(group.Members))
	for i, member := range group.Members {
		members[i] = &pb.GetGroupResponse_GroupMember{
			Guid:     member.MemberGUID,
			Flags:    uint32(member.MemberFlags),
			Name:     member.MemberName,
			IsOnline: member.IsOnline,
			SubGroup: uint32(member.SubGroup),
			Roles:    uint32(member.Roles),
			RealmID:  member.RealmID,
		}
	}

	return &pb.GetGroupResponse{
		Api: groupserver.Ver,
		Group: &pb.GetGroupResponse_Group{
			Id:              uint32(group.ID),
			Leader:          group.LeaderGUID,
			LootMethod:      uint32(group.LootMethod),
			Looter:          group.LooterGUID,
			LootThreshold:   uint32(group.LootThreshold),
			GroupType:       uint32(group.GroupType),
			Difficulty:      uint32(group.Difficulty),
			RaidDifficulty:  uint32(group.RaidDifficulty),
			MasterLooter:    group.MasterLooterGuid,
			Members:         members,
			TargetIconsList: group.TargetIcons[:],
			RealmID:         group.RealmID,
		},
	}
}

func (g GroupServer) StartReadyCheck(ctx context.Context, params *pb.StartReadyCheckRequest) (*pb.StartReadyCheckResponse, error) {
	err := g.groupService.StartReadyCheck(ctx, params.RealmID, params.LeaderGUID, params.DurationMs)
	if err != nil {
		return nil, err
	}

	return &pb.StartReadyCheckResponse{Api: groupserver.Ver}, nil
}

func (g GroupServer) SetReadyCheckMemberState(ctx context.Context, params *pb.SetReadyCheckMemberStateRequest) (*pb.SetReadyCheckMemberStateResponse, error) {
	err := g.groupService.SetReadyCheckMemberState(ctx, params.RealmID, params.MemberGUID, uint8(params.State))
	if err != nil {
		return nil, err
	}

	return &pb.SetReadyCheckMemberStateResponse{Api: groupserver.Ver}, nil
}

func (g GroupServer) FinishReadyCheck(ctx context.Context, params *pb.FinishReadyCheckRequest) (*pb.FinishReadyCheckResponse, error) {
	err := g.groupService.FinishReadyCheck(ctx, params.RealmID, params.PlayerGUID)
	if err != nil {
		return nil, err
	}

	return &pb.FinishReadyCheckResponse{Api: groupserver.Ver}, nil
}

func (g GroupServer) ChangeMemberSubGroup(ctx context.Context, params *pb.ChangeMemberSubGroupRequest) (*pb.ChangeMemberSubGroupResponse, error) {
	err := g.groupService.ChangeMemberSubGroup(ctx, params.RealmID, params.UpdaterGUID, params.MemberGUID, uint8(params.SubGroup))
	if err != nil {
		return nil, err
	}

	return &pb.ChangeMemberSubGroupResponse{Api: groupserver.Ver}, nil
}

func (g GroupServer) SetMemberFlags(ctx context.Context, params *pb.SetMemberFlagsRequest) (*pb.SetMemberFlagsResponse, error) {
	err := g.groupService.SetMemberFlags(ctx, params.RealmID, params.UpdaterGUID, params.MemberGUID, uint8(params.Flags), uint8(params.Roles))
	if err != nil {
		return nil, err
	}

	return &pb.SetMemberFlagsResponse{Api: groupserver.Ver}, nil
}

func (g GroupServer) RegisterAcceptedLfgGroup(ctx context.Context, params *pb.RegisterAcceptedLfgGroupRequest) (*pb.RegisterAcceptedLfgGroupResponse, error) {
	members := make([]service.AcceptedLfgGroupMember, 0, len(params.Members))
	for _, member := range params.Members {
		if member == nil {
			continue
		}
		members = append(members, service.AcceptedLfgGroupMember{
			RealmID:            member.RealmID,
			PlayerGUID:         member.PlayerGUID,
			SelectedRoles:      uint8(member.SelectedRoles),
			AssignedRole:       uint8(member.AssignedRole),
			QueueLeaderRealmID: member.QueueLeaderRealmID,
			QueueLeaderGUID:    member.QueueLeaderGUID,
		})
	}

	groupID, err := g.groupService.RegisterAcceptedLfgGroup(
		ctx,
		params.RealmID,
		params.ProposalID,
		params.DungeonEntry,
		params.LeaderRealmID,
		params.LeaderGUID,
		params.CrossRealm,
		members,
	)
	if err != nil {
		return nil, err
	}

	return &pb.RegisterAcceptedLfgGroupResponse{Api: groupserver.Ver, GroupID: uint32(groupID)}, nil
}

func (g GroupServer) RegisterMaterializedLfgGroup(ctx context.Context, params *pb.RegisterMaterializedLfgGroupRequest) (*pb.RegisterMaterializedLfgGroupResponse, error) {
	members := make([]service.MaterializedLfgGroupMember, 0, len(params.Members))
	for _, member := range params.Members {
		if member == nil {
			continue
		}
		members = append(members, service.MaterializedLfgGroupMember{
			RealmID:    member.RealmID,
			PlayerGUID: member.PlayerGUID,
			Name:       member.Name,
			Online:     member.IsOnline,
			Flags:      uint8(member.Flags),
			Roles:      uint8(member.Roles),
			SubGroup:   uint8(member.SubGroup),
		})
	}

	err := g.groupService.RegisterMaterializedLfgGroup(
		ctx,
		params.RealmID,
		uint(params.GroupID),
		params.LeaderGUID,
		uint8(params.GroupType),
		uint8(params.Difficulty),
		uint8(params.RaidDifficulty),
		members,
	)
	if err != nil {
		return nil, err
	}

	return &pb.RegisterMaterializedLfgGroupResponse{Api: groupserver.Ver}, nil
}

func (g GroupServer) UpdateMemberState(ctx context.Context, params *pb.UpdateMemberStateRequest) (*pb.UpdateMemberStateResponse, error) {
	err := g.groupService.UpdateMemberState(
		ctx,
		params.RealmID,
		params.MemberGUID,
		params.Online,
		uint8(params.Level),
		uint8(params.ClassID),
		params.ZoneID,
		params.MapID,
		params.Health,
		params.MaxHealth,
		uint8(params.PowerType),
		params.Power,
		params.MaxPower,
		params.InstanceID,
	)
	if err != nil {
		return nil, err
	}

	return &pb.UpdateMemberStateResponse{Api: groupserver.Ver}, nil
}

func (g GroupServer) BulkUpdateMemberStates(ctx context.Context, params *pb.BulkUpdateMemberStatesRequest) (*pb.BulkUpdateMemberStatesResponse, error) {
	snapshots := make([]service.MemberStateSnapshot, 0, len(params.Snapshots))
	for _, snapshot := range params.Snapshots {
		if snapshot == nil {
			continue
		}

		snapshots = append(snapshots, service.MemberStateSnapshot{
			MemberGUID:  snapshot.MemberGUID,
			Online:      snapshot.Online,
			Level:       uint8(snapshot.Level),
			Class:       uint8(snapshot.ClassID),
			ZoneID:      snapshot.ZoneID,
			MapID:       snapshot.MapID,
			Health:      snapshot.Health,
			MaxHealth:   snapshot.MaxHealth,
			PowerType:   uint8(snapshot.PowerType),
			Power:       snapshot.Power,
			MaxPower:    snapshot.MaxPower,
			InstanceID:  snapshot.InstanceID,
			AurasKnown:  snapshot.AurasKnown,
			Auras:       protoMemberAuras(snapshot.Auras),
			TimestampMs: snapshot.TimestampMs,
			Dead:        snapshot.Dead,
			Ghost:       snapshot.Ghost,
		})
	}

	err := g.groupService.BulkUpdateMemberStates(
		ctx,
		params.RealmID,
		params.SourceGatewayID,
		params.SourceWorldserverID,
		snapshots,
	)
	if err != nil {
		return nil, err
	}

	return &pb.BulkUpdateMemberStatesResponse{Api: groupserver.Ver}, nil
}

func protoMemberAuras(auras []*pb.PlayerAuraSnapshot) []service.MemberAuraState {
	if len(auras) == 0 {
		return nil
	}

	out := make([]service.MemberAuraState, 0, len(auras))
	for _, aura := range auras {
		if aura == nil {
			continue
		}
		out = append(out, service.MemberAuraState{
			Slot:    uint8(aura.Slot),
			SpellID: aura.SpellID,
			Flags:   uint8(aura.Flags),
		})
	}

	return out
}

func (g GroupServer) ResetInstance(ctx context.Context, params *pb.ResetInstanceRequest) (*pb.ResetInstanceResponse, error) {
	err := g.groupService.ResetInstance(ctx, params.RealmID, params.PlayerGUID, params.MapID, uint8(params.Difficulty))
	if err != nil {
		return nil, err
	}

	return &pb.ResetInstanceResponse{Api: groupserver.Ver}, nil
}

func (g GroupServer) SetInstanceBindExtension(ctx context.Context, params *pb.SetInstanceBindExtensionRequest) (*pb.SetInstanceBindExtensionResponse, error) {
	err := g.groupService.SetInstanceBindExtension(ctx, params.RealmID, params.PlayerGUID, params.MapID, uint8(params.Difficulty), params.Extended)
	if err != nil {
		return nil, err
	}

	return &pb.SetInstanceBindExtensionResponse{Api: groupserver.Ver}, nil
}
