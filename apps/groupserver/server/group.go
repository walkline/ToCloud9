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
	groupID, err := g.groupService.GroupIDByPlayer(ctx, request.RealmID, request.Player)
	if err != nil {
		return nil, err
	}

	return &pb.GetGroupIDByPlayerResponse{
		Api:     groupserver.Ver,
		GroupID: uint32(groupID),
	}, nil
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
			Roles:    uint32(member.SubGroup),
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
		},
	}
}
