package server

import (
	"context"
	"errors"

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
	lfgService  service.LFGService
	grpcConnMgr conn.GameServerGRPCConnMgr
}

func NewMatchmakingServer(bgService service.BattleGroundService, lfgService service.LFGService, grpcConnMgr conn.GameServerGRPCConnMgr) pb.MatchmakingServiceServer {
	return &MatchmakingServer{
		bgService:   bgService,
		lfgService:  lfgService,
		grpcConnMgr: grpcConnMgr,
	}
}

func (s *MatchmakingServer) EnqueueToBattleground(ctx context.Context, req *pb.EnqueueToBattlegroundRequest) (*pb.EnqueueToBattlegroundResponse, error) {
	err := s.bgService.AddGroupToQueue(ctx, req.RealmID, req.LeaderGUID, req.PartyMembers, battleground.QueueTypeID(req.BgTypeID), uint8(req.LeadersLvl), battleground.PVPTeam(req.TeamID), uint8(req.ArenaType), req.IsRated)
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
		PlayerUnwrapped: guid.PlayerUnwrapped{
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
			_, assignedTeam := bg.TeamForInvitedPlayer(req.PlayerGUID, req.RealmID)
			slots[i] = &pb.BattlegroundQueueDataForPlayerResponse_QueueSlot{
				BgTypeID: uint32(bg.BattlegroundTypeID),
				Status:   pb.PlayerQueueStatus_Invited,
				AssignedBattlegroundData: &pb.BattlegroundQueueDataForPlayerResponse_AssignedBattlegroundData{
					AssignedBattlegroundInstanceID: bg.InstanceID,
					MapID:                          bg.MapID,
					BattlegroupID:                  bg.BattleGroupID,
					GameserverAddress:              bg.GameserverAddress,
					GameserverGRPCAddress:          s.grpcConnMgr.GRPCAddressForGameServer(bg.GameserverAddress),
					AssignedTeamID:                 pb.PVPTeamID(assignedTeam),
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

func (s *MatchmakingServer) FinishRatedArenaMatch(ctx context.Context, request *pb.FinishRatedArenaMatchRequest) (*pb.FinishRatedArenaMatchResponse, error) {
	participants := make([]service.RatedArenaParticipant, 0, len(request.GetParticipants()))
	for _, participant := range request.GetParticipants() {
		participants = append(participants, service.RatedArenaParticipant{
			Team:       battlegroundTeamFromProto(participant.GetTeam()),
			PlayerGUID: participant.GetPlayerGUID(),
		})
	}

	result, err := s.bgService.FinishRatedArenaMatch(ctx, service.RatedArenaMatchResultRequest{
		OwnerRealmID:                  request.GetOwnerRealmID(),
		IsCrossRealm:                  request.GetIsCrossRealm(),
		InstanceID:                    request.GetInstanceID(),
		ArenaType:                     uint8(request.GetArenaType()),
		WinnerTeam:                    battlegroundTeamFromProto(request.GetWinnerTeam()),
		ValidArena:                    request.GetValidArena(),
		AllianceArenaTeamID:           request.GetAllianceArenaTeamID(),
		HordeArenaTeamID:              request.GetHordeArenaTeamID(),
		AllianceArenaMatchmakerRating: request.GetAllianceArenaMatchmakerRating(),
		HordeArenaMatchmakerRating:    request.GetHordeArenaMatchmakerRating(),
		Participants:                  participants,
	})
	if err != nil {
		return &pb.FinishRatedArenaMatchResponse{
			Api:    matchmaking.Ver,
			Status: arenaTeamMutationStatusForError(err),
		}, nil
	}

	memberResults := make([]*pb.RatedArenaMemberResult, 0, len(result.MemberResults))
	for _, member := range result.MemberResults {
		memberResults = append(memberResults, &pb.RatedArenaMemberResult{
			Team:             protoPVPTeam(member.Team),
			PlayerGUID:       member.PlayerGUID,
			PersonalRating:   member.PersonalRating,
			WeekGames:        member.WeekGames,
			SeasonGames:      member.SeasonGames,
			WeekWins:         member.WeekWins,
			SeasonWins:       member.SeasonWins,
			MatchmakerRating: member.MatchmakerRating,
		})
	}

	return &pb.FinishRatedArenaMatchResponse{
		Api:           matchmaking.Ver,
		Status:        pb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_OK,
		AllianceScore: ratedArenaScoreToProto(result.AllianceScore),
		HordeScore:    ratedArenaScoreToProto(result.HordeScore),
		MemberResults: memberResults,
	}, nil
}

func battlegroundTeamFromProto(team pb.PVPTeamID) battleground.PVPTeam {
	switch team {
	case pb.PVPTeamID_Alliance:
		return battleground.TeamAlliance
	case pb.PVPTeamID_Horde:
		return battleground.TeamHorde
	default:
		return battleground.TeamAny
	}
}

func protoPVPTeam(team battleground.PVPTeam) pb.PVPTeamID {
	switch team {
	case battleground.TeamAlliance:
		return pb.PVPTeamID_Alliance
	case battleground.TeamHorde:
		return pb.PVPTeamID_Horde
	default:
		return pb.PVPTeamID_Any
	}
}

func ratedArenaScoreToProto(score service.RatedArenaTeamScore) *pb.RatedArenaTeamScore {
	return &pb.RatedArenaTeamScore{
		RealmID:          score.RealmID,
		ArenaTeamID:      score.ArenaTeamID,
		TeamName:         score.TeamName,
		RatingChange:     score.RatingChange,
		MatchmakerRating: score.MatchmakerRating,
	}
}

func arenaTeamMutationStatusForError(err error) pb.MatchmakingArenaTeamMutationStatus {
	switch {
	case errors.Is(err, repo.ErrArenaTeamNotFound):
		return pb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_NOT_FOUND
	case errors.Is(err, repo.ErrArenaTeamMemberMismatch):
		return pb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_MEMBER_MISMATCH
	case errors.Is(err, repo.ErrArenaTeamInvalidType):
		return pb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_INVALID_TYPE
	case errors.Is(err, service.ErrInvalidArenaType):
		return pb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_INVALID_TYPE
	default:
		return pb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_FAILED
	}
}

func (s *MatchmakingServer) JoinLfg(ctx context.Context, req *pb.JoinLfgRequest) (*pb.JoinLfgResponse, error) {
	members := make([]service.LFGMember, 0, len(req.Members))
	for _, member := range req.Members {
		realmID := member.RealmID
		if realmID == 0 {
			realmID = req.RealmID
		}
		members = append(members, service.LFGMember{
			RealmID:       realmID,
			PlayerGUID:    member.PlayerGUID,
			Roles:         uint8(member.Roles),
			Leader:        member.Leader,
			WorldserverID: member.WorldserverID,
		})
	}

	status, err := s.lfgService.JoinLfg(ctx, service.LFGJoinData{
		RealmID:        req.RealmID,
		LeaderGUID:     req.LeaderGUID,
		Members:        members,
		DungeonEntries: req.DungeonEntries,
		Comment:        req.Comment,
	})
	if err != nil {
		return &pb.JoinLfgResponse{
			Api:    matchmaking.Ver,
			Result: lfgJoinResultForError(err),
			Status: lfgStatusToProto(&service.LFGStatus{State: service.LFGStateNone}),
		}, nil
	}

	return &pb.JoinLfgResponse{
		Api:    matchmaking.Ver,
		Result: pb.LfgJoinResult_LFG_JOIN_OK,
		Status: lfgStatusToProto(status),
	}, nil
}

func lfgJoinResultForError(err error) pb.LfgJoinResult {
	switch {
	case errors.Is(err, service.ErrLFGInvalidDungeon):
		return pb.LfgJoinResult_LFG_JOIN_DUNGEON_INVALID
	case errors.Is(err, service.ErrLFGGroupFull):
		return pb.LfgJoinResult_LFG_JOIN_TOO_MANY_MEMBERS
	case errors.Is(err, service.ErrLFGInvalidMember),
		errors.Is(err, service.ErrLFGInvalidRoles),
		errors.Is(err, service.ErrLFGAlreadyQueuedOrMatched):
		return pb.LfgJoinResult_LFG_JOIN_FAILED
	case errors.Is(err, service.ErrLFGMultiRealm):
		return pb.LfgJoinResult_LFG_JOIN_MULTI_REALM
	default:
		return pb.LfgJoinResult_LFG_JOIN_INTERNAL_ERROR
	}
}

func (s *MatchmakingServer) LeaveLfg(ctx context.Context, req *pb.LeaveLfgRequest) (*pb.LeaveLfgResponse, error) {
	if err := s.lfgService.LeaveLfg(ctx, req.RealmID, req.PlayerGUID); err != nil {
		if errors.Is(err, service.ErrLFGNotFound) || errors.Is(err, service.ErrLFGNotLeader) {
			return &pb.LeaveLfgResponse{
				Api: matchmaking.Ver,
			}, nil
		}
		return nil, err
	}

	return &pb.LeaveLfgResponse{
		Api: matchmaking.Ver,
	}, nil
}

func (s *MatchmakingServer) SetLfgRoles(ctx context.Context, req *pb.SetLfgRolesRequest) (*pb.SetLfgRolesResponse, error) {
	status, err := s.lfgService.SetLfgRoles(ctx, req.RealmID, req.PlayerGUID, uint8(req.Roles))
	if err != nil {
		return nil, err
	}

	return &pb.SetLfgRolesResponse{
		Api:    matchmaking.Ver,
		Status: lfgStatusToProto(status),
	}, nil
}

func (s *MatchmakingServer) AnswerLfgProposal(ctx context.Context, req *pb.AnswerLfgProposalRequest) (*pb.AnswerLfgProposalResponse, error) {
	status, err := s.lfgService.AnswerLfgProposal(ctx, req.RealmID, req.PlayerGUID, req.ProposalID, req.Accept)
	if err != nil {
		return nil, err
	}

	return &pb.AnswerLfgProposalResponse{
		Api:    matchmaking.Ver,
		Status: lfgStatusToProto(status),
	}, nil
}

func (s *MatchmakingServer) LfgStatus(ctx context.Context, req *pb.LfgStatusRequest) (*pb.LfgStatusResponse, error) {
	status, err := s.lfgService.LfgStatus(ctx, req.RealmID, req.PlayerGUID)
	if err != nil {
		return nil, err
	}

	return &pb.LfgStatusResponse{
		Api:    matchmaking.Ver,
		Status: lfgStatusToProto(status),
	}, nil
}

func (s *MatchmakingServer) CompleteLfgDungeon(ctx context.Context, req *pb.CompleteLfgDungeonRequest) (*pb.CompleteLfgDungeonResponse, error) {
	players := make([]service.LFGPlayerKey, 0, len(req.Players))
	for _, player := range req.Players {
		if player.GetRealmID() == 0 || player.GetPlayerGUID() == 0 {
			continue
		}
		players = append(players, service.LFGPlayerKey{
			RealmID:    player.GetRealmID(),
			PlayerGUID: player.GetPlayerGUID(),
		})
	}
	if err := s.lfgService.CompleteLfgDungeon(ctx, req.GetCompletedDungeonEntry(), req.GetSelectedDungeonEntry(), players); err != nil {
		return nil, err
	}

	return &pb.CompleteLfgDungeonResponse{
		Api: matchmaking.Ver,
	}, nil
}

func lfgStatusToProto(status *service.LFGStatus) *pb.LfgStatusData {
	if status == nil {
		status = &service.LFGStatus{State: service.LFGStateNone}
	}

	queuedMembers := make([]*pb.LfgMember, 0, len(status.QueuedMembers))
	for _, member := range status.QueuedMembers {
		queuedMembers = append(queuedMembers, &pb.LfgMember{
			RealmID:            member.RealmID,
			PlayerGUID:         member.PlayerGUID,
			Roles:              uint32(member.Roles),
			Leader:             member.Leader,
			WorldserverID:      member.WorldserverID,
			QueueLeaderRealmID: member.QueueLeaderRealmID,
			QueueLeaderGUID:    member.QueueLeaderGUID,
		})
	}

	proposalMembers := make([]*pb.LfgProposalMember, 0, len(status.ProposalMembers))
	for _, member := range status.ProposalMembers {
		proposalMembers = append(proposalMembers, &pb.LfgProposalMember{
			RealmID:       member.RealmID,
			PlayerGUID:    member.PlayerGUID,
			SelectedRoles: uint32(member.SelectedRoles),
			AssignedRole:  uint32(member.AssignedRole),
			Answered:      member.Answered,
			Accepted:      member.Accepted,
		})
	}

	return &pb.LfgStatusData{
		State:                  pb.LfgState(status.State),
		ProposalID:             status.ProposalID,
		ProposalState:          pb.LfgProposalState(status.ProposalState),
		DungeonEntry:           status.DungeonEntry,
		SelectedDungeons:       status.SelectedDungeons,
		QueuedMembers:          queuedMembers,
		ProposalMembers:        proposalMembers,
		QueuedTimeMilliseconds: status.QueuedTimeMilliseconds,
		TanksNeeded:            uint32(status.TanksNeeded),
		HealersNeeded:          uint32(status.HealersNeeded),
		DamageNeeded:           uint32(status.DamageNeeded),
	}
}
