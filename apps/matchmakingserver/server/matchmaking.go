package server

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	matchmaking "github.com/walkline/ToCloud9/apps/matchmakingserver"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/battleground"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/lfg"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/repo"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/service"
	"github.com/walkline/ToCloud9/gen/matchmaking/pb"
	"github.com/walkline/ToCloud9/shared/gameserver/conn"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

type MatchmakingServer struct {
	pb.UnimplementedMatchmakingServiceServer

	bgService   service.BattleGroundService
	lfgService  *lfg.Service
	grpcConnMgr conn.GameServerGRPCConnMgr
}

func NewMatchmakingServer(bgService service.BattleGroundService, lfgService *lfg.Service, grpcConnMgr conn.GameServerGRPCConnMgr) pb.MatchmakingServiceServer {
	return &MatchmakingServer{
		bgService:   bgService,
		lfgService:  lfgService,
		grpcConnMgr: grpcConnMgr,
	}
}

func (s *MatchmakingServer) EnqueueToBattleground(ctx context.Context, req *pb.EnqueueToBattlegroundRequest) (*pb.EnqueueToBattlegroundResponse, error) {
	var queuedAt time.Time
	if req.QueuedAtUnixMilli > 0 {
		queuedAt = time.UnixMilli(req.QueuedAtUnixMilli)
	}
	err := s.bgService.AddGroupToQueue(ctx, req.RealmID, req.LeaderGUID, req.PartyMembers, battleground.QueueTypeID(req.BgTypeID), uint8(req.LeadersLvl), battleground.PVPTeam(req.TeamID), queuedAt)
	if err != nil {
		// Expected business failures get a typed code so the gateway can show
		// them to the player; everything else stays internal.
		if errors.Is(err, service.ErrAlreadyInQueue) {
			return nil, status.Error(codes.FailedPrecondition, service.ErrAlreadyInQueue.Error())
		}
		return nil, err
	}
	return &pb.EnqueueToBattlegroundResponse{
		Api:        matchmaking.Ver,
		InstanceID: s.lfgService.InstanceID(),
	}, nil
}

func (s *MatchmakingServer) RemovePlayerFromQueue(ctx context.Context, req *pb.RemovePlayerFromQueueRequest) (*pb.RemovePlayerFromQueueResponse, error) {
	err := s.bgService.RemovePlayerFromQueue(ctx, req.PlayerGUID, req.RealmID, battleground.QueueTypeID(req.BattlegroundType))
	if err != nil {
		return nil, err
	}
	return &pb.RemovePlayerFromQueueResponse{
		Api: matchmaking.Ver,
	}, nil
}

func (s *MatchmakingServer) BattlegroundQueueDataForPlayer(ctx context.Context, req *pb.BattlegroundQueueDataForPlayerRequest) (*pb.BattlegroundQueueDataForPlayerResponse, error) {
	links := s.bgService.GetQueueOrBattlegroundLinkForPlayer(service.QueuesByRealmAndPlayerKey{
		guid.PlayerUnwrapped{
			RealmID: uint16(req.RealmID),
			LowGUID: guid.LowType(req.PlayerGUID),
		},
	})

	slots := make([]*pb.BattlegroundQueueDataForPlayerResponse_QueueSlot, len(links))
	for i, link := range links {
		if link.BattlegroundKey != nil {
			bg, err := s.bgService.GetBattlegroundByBattlegroundKey(ctx, link.BattlegroundKey.InstanceID, repo.RealmWithBattlegroupKey{
				RealmID:       link.BattlegroundKey.RealmID,
				BattlegroupID: link.BattlegroundKey.BattlegroupID,
			})
			if err != nil {
				return nil, err
			}
			slots[i] = &pb.BattlegroundQueueDataForPlayerResponse_QueueSlot{
				BgTypeID: uint32(bg.BattlegroundTypeID),
				Status:   pb.PlayerQueueStatus_Invited,
				AssignedBattlegroundData: &pb.BattlegroundQueueDataForPlayerResponse_AssignedBattlegroundData{
					AssignedBattlegroundInstanceID: bg.InstanceID,
					MapID:                          bg.MapID,
					BattlegroupID:                  bg.BattleGroupID,
					GameserverAddress:              bg.GameserverAddress,
					GameserverGRPCAddress:          s.grpcConnMgr.GRPCAddressForGameServer(bg.GameserverAddress),
				},
			}
		} else {
			slots[i] = &pb.BattlegroundQueueDataForPlayerResponse_QueueSlot{
				BgTypeID: uint32(link.Queue.GetQueueTypeID()),
				Status:   pb.PlayerQueueStatus_InQueue,
			}
		}
	}

	return &pb.BattlegroundQueueDataForPlayerResponse{
		Api:   matchmaking.Ver,
		Slots: slots,
	}, nil
}

func (s *MatchmakingServer) PlayerLeftBattleground(ctx context.Context, request *pb.PlayerLeftBattlegroundRequest) (*pb.PlayerLeftBattlegroundResponse, error) {
	err := s.bgService.PlayerLeftBattleground(ctx, request.PlayerGUID, request.RealmID, request.InstanceID, request.IsCrossRealm)
	if err != nil {
		return nil, err
	}

	return &pb.PlayerLeftBattlegroundResponse{
		Api: matchmaking.Ver,
	}, nil
}

func (s *MatchmakingServer) PlayerJoinedBattleground(ctx context.Context, request *pb.PlayerJoinedBattlegroundRequest) (*pb.PlayerJoinedBattlegroundResponse, error) {
	err := s.bgService.PlayerJoinedBattleground(ctx, request.PlayerGUID, request.RealmID, request.InstanceID, request.IsCrossRealm)
	if err != nil {
		return nil, err
	}

	return &pb.PlayerJoinedBattlegroundResponse{
		Api: matchmaking.Ver,
	}, nil
}

func (s *MatchmakingServer) BattlegroundStatusChanged(ctx context.Context, request *pb.BattlegroundStatusChangedRequest) (*pb.BattlegroundStatusChangedResponse, error) {
	err := s.bgService.BattlegroundStatusChanged(ctx, battleground.Status(request.Status), request.RealmID, request.InstanceID, request.IsCrossRealm)
	if err != nil {
		return nil, err
	}

	return &pb.BattlegroundStatusChangedResponse{
		Api: matchmaking.Ver,
	}, nil
}

func (s *MatchmakingServer) JoinLFG(_ context.Context, request *pb.JoinLFGRequest) (*pb.JoinLFGResponse, error) {
	entry := &lfg.Entry{
		RequestID:        request.RequestID,
		BattlegroupID:    request.BattlegroupID,
		Leader:           lfg.PlayerKey{RealmID: leaderRealmID(request), GUID: request.LeaderGUID},
		SelectedDungeons: uint32Set(request.SelectedDungeonIDs),
	}
	if request.QueuedAtUnixMilli > 0 {
		entry.QueuedAt = time.UnixMilli(request.QueuedAtUnixMilli)
	}
	entry.Members = make([]lfg.Member, 0, len(request.Members))
	for _, member := range request.Members {
		entry.Members = append(entry.Members, lfg.Member{
			PlayerKey:        lfg.PlayerKey{RealmID: member.RealmID, GUID: member.PlayerGUID},
			Roles:            lfg.Role(member.Roles),
			Level:            uint8(member.Level),
			Class:            uint8(member.ClassID),
			EligibleDungeons: uint32Set(member.EligibleDungeonIDs),
		})
	}

	proposal, err := s.lfgService.Join(entry)
	if err != nil {
		switch {
		case errors.Is(err, lfg.ErrInvalidEntry):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, lfg.ErrPlayerAlreadyQueued):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		default:
			return nil, err
		}
	}
	response := &pb.JoinLFGResponse{
		Api:        matchmaking.Ver,
		InstanceID: s.lfgService.InstanceID(),
		Status:     pb.LFGQueueStatus_LFGQueueStatusQueued,
	}
	if proposal != nil {
		response.Status = pb.LFGQueueStatus_LFGQueueStatusProposed
		response.ProposalID = proposal.ID
	}
	return response, nil
}

func (s *MatchmakingServer) LeaveLFG(_ context.Context, request *pb.LeaveLFGRequest) (*pb.LeaveLFGResponse, error) {
	err := s.lfgService.Leave(lfg.PlayerKey{RealmID: request.RealmID, GUID: request.PlayerGUID})
	if err != nil && !errors.Is(err, lfg.ErrPlayerNotQueued) {
		return nil, err
	}
	return &pb.LeaveLFGResponse{Api: matchmaking.Ver, InstanceID: s.lfgService.InstanceID()}, nil
}

func (s *MatchmakingServer) GetLFGStatus(_ context.Context, request *pb.GetLFGStatusRequest) (*pb.GetLFGStatusResponse, error) {
	playerStatus := s.lfgService.Status(lfg.PlayerKey{RealmID: request.RealmID, GUID: request.PlayerGUID})
	response := &pb.GetLFGStatusResponse{
		Api:          matchmaking.Ver,
		InstanceID:   s.lfgService.InstanceID(),
		Status:       pb.LFGQueueStatus(playerStatus.Status),
		ProposalID:   playerStatus.ProposalID,
		DungeonID:    playerStatus.DungeonID,
		AssignedRole: pb.LFGRole(playerStatus.AssignedRole),
	}
	if !playerStatus.QueuedAt.IsZero() {
		response.QueuedAtUnixMilli = playerStatus.QueuedAt.UnixMilli()
	}
	return response, nil
}

func leaderRealmID(request *pb.JoinLFGRequest) uint32 {
	for _, member := range request.Members {
		if member.PlayerGUID == request.LeaderGUID {
			return member.RealmID
		}
	}
	return 0
}

func uint32Set(values []uint32) map[uint32]struct{} {
	result := make(map[uint32]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
