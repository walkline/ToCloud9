package server

import (
	"context"

	"github.com/walkline/ToCloud9/apps/groupserver"
	"github.com/walkline/ToCloud9/apps/groupserver/service"
	"github.com/walkline/ToCloud9/gen/group/pb"
)

type GroupServer struct {
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
			Id:             uint32(group.ID),
			Leader:         group.LeaderGUID,
			LootMethod:     uint32(group.LootMethod),
			Looter:         group.LooterGUID,
			LootThreshold:  uint32(group.LootThreshold),
			GroupType:      uint32(group.GroupType),
			Difficulty:     uint32(group.Difficulty),
			RaidDifficulty: uint32(group.RaidDifficulty),
			MasterLooter:   group.MasterLooterGuid,
			Members:        members,
		},
	}, nil
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
	err := g.groupService.AcceptInvite(ctx, params.RealmID, params.Player)
	if err != nil {
		return nil, err
	}

	status := pb.AcceptInviteResponse_Ok

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
