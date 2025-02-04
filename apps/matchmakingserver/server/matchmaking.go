package server

import (
	"context"

	matchmaking "github.com/walkline/ToCloud9/apps/matchmakingserver"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/battleground"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/repo"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/service"
	"github.com/walkline/ToCloud9/gen/matchmaking/pb"
	"github.com/walkline/ToCloud9/shared/gameserver/conn"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

type MatchmakingServer struct {
	pb.UnimplementedMatchmakingServiceServer

	bgService   service.BattleGroundService
	grpcConnMgr conn.GameServerGRPCConnMgr
}

func NewMatchmakingServer(bgService service.BattleGroundService, grpcConnMgr conn.GameServerGRPCConnMgr) pb.MatchmakingServiceServer {
	return &MatchmakingServer{
		bgService:   bgService,
		grpcConnMgr: grpcConnMgr,
	}
}

func (s *MatchmakingServer) EnqueueToBattleground(ctx context.Context, req *pb.EnqueueToBattlegroundRequest) (*pb.EnqueueToBattlegroundResponse, error) {
	err := s.bgService.AddGroupToQueue(ctx, req.RealmID, req.LeaderGUID, req.PartyMembers, battleground.QueueTypeID(req.BgTypeID), uint8(req.LeadersLvl), battleground.PVPTeam(req.TeamID))
	if err != nil {
		return nil, err
	}
	return &pb.EnqueueToBattlegroundResponse{
		Api: matchmaking.Ver,
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
