package repo

import (
	"context"
	"errors"
	"fmt"

	charserver "github.com/walkline/ToCloud9/apps/charserver"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
)

var (
	ErrArenaTeamNotFound       = errors.New("arena team not found")
	ErrArenaTeamMemberMismatch = errors.New("arena team member mismatch")
	ErrArenaTeamPartySize      = errors.New("invalid arena team party size")
	ErrArenaTeamInvalidType    = errors.New("invalid arena team type")
)

type ArenaTeamQueueData struct {
	ArenaTeamID             uint32
	TeamRating              uint32
	MatchmakerRating        uint32
	PreviousOpponentsTeamID uint32
}

type ArenaTeamSaveStatsRequest struct {
	RealmID     uint32
	ArenaTeamID uint32
	Rating      uint32
	WeekGames   uint32
	WeekWins    uint32
	SeasonGames uint32
	SeasonWins  uint32
	Rank        uint32
	Slot        uint32
	Members     []ArenaTeamSaveStatsMember
}

type ArenaTeamDetails struct {
	RealmID     uint32
	ArenaTeamID uint32
	Name        string
	Type        uint8
	Rating      uint32
	WeekGames   uint32
	WeekWins    uint32
	SeasonGames uint32
	SeasonWins  uint32
	Rank        uint32
	Members     []ArenaTeamDetailsMember
}

type ArenaTeamDetailsMember struct {
	PlayerGUID       uint64
	PersonalRating   uint32
	WeekGames        uint32
	WeekWins         uint32
	SeasonGames      uint32
	SeasonWins       uint32
	MatchmakerRating uint32
	MaxMMR           uint32
}

type ArenaTeamSaveStatsMember struct {
	PlayerGUID       uint64
	PersonalRating   uint32
	WeekGames        uint32
	WeekWins         uint32
	SeasonGames      uint32
	SeasonWins       uint32
	MatchmakerRating uint32
	MaxMMR           uint32
	SaveArenaStats   bool
}

type ArenaTeamRepository interface {
	QueueDataForRatedArena(ctx context.Context, realmID uint32, leaderGUID uint64, playerGUIDs []uint64, arenaType uint8, startMatchmakerRating uint32) (*ArenaTeamQueueData, error)
	GetTeam(ctx context.Context, realmID uint32, arenaTeamID uint32) (*ArenaTeamDetails, error)
	SaveStats(ctx context.Context, request ArenaTeamSaveStatsRequest) error
}

type charserverArenaTeamRepo struct {
	client pbChar.CharactersServiceClient
}

func NewCharserverArenaTeamRepo(client pbChar.CharactersServiceClient) ArenaTeamRepository {
	return &charserverArenaTeamRepo{client: client}
}

func (r *charserverArenaTeamRepo) QueueDataForRatedArena(ctx context.Context, realmID uint32, leaderGUID uint64, playerGUIDs []uint64, arenaType uint8, startMatchmakerRating uint32) (*ArenaTeamQueueData, error) {
	if r.client == nil {
		return nil, errors.New("characters service client is nil")
	}

	resp, err := r.client.ArenaTeamQueueDataForRatedArena(ctx, &pbChar.ArenaTeamQueueDataForRatedArenaRequest{
		Api:                   charserver.Ver,
		RealmID:               realmID,
		LeaderGUID:            leaderGUID,
		PlayerGUIDs:           playerGUIDs,
		ArenaType:             uint32(arenaType),
		StartMatchmakerRating: startMatchmakerRating,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("characters service returned nil arena team queue data response")
	}

	switch resp.GetStatus() {
	case pbChar.ArenaTeamQueueDataForRatedArenaResponse_Ok:
		return &ArenaTeamQueueData{
			ArenaTeamID:             resp.GetArenaTeamID(),
			TeamRating:              resp.GetTeamRating(),
			MatchmakerRating:        resp.GetMatchmakerRating(),
			PreviousOpponentsTeamID: resp.GetPreviousOpponentsTeamID(),
		}, nil
	case pbChar.ArenaTeamQueueDataForRatedArenaResponse_NotFound:
		return nil, ErrArenaTeamNotFound
	case pbChar.ArenaTeamQueueDataForRatedArenaResponse_MemberMismatch:
		return nil, ErrArenaTeamMemberMismatch
	case pbChar.ArenaTeamQueueDataForRatedArenaResponse_InvalidPartySize:
		return nil, ErrArenaTeamPartySize
	case pbChar.ArenaTeamQueueDataForRatedArenaResponse_InvalidType:
		return nil, ErrArenaTeamInvalidType
	default:
		return nil, fmt.Errorf("characters service failed arena team queue lookup with status %s", resp.GetStatus().String())
	}
}

func (r *charserverArenaTeamRepo) GetTeam(ctx context.Context, realmID uint32, arenaTeamID uint32) (*ArenaTeamDetails, error) {
	if r.client == nil {
		return nil, errors.New("characters service client is nil")
	}

	resp, err := r.client.GetArenaTeam(ctx, &pbChar.GetArenaTeamRequest{
		Api:         charserver.Ver,
		RealmID:     realmID,
		ArenaTeamID: arenaTeamID,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("characters service returned nil arena team response")
	}

	switch resp.GetStatus() {
	case pbChar.GetArenaTeamResponse_Ok:
		team := resp.GetTeam()
		if team == nil {
			return nil, ErrArenaTeamNotFound
		}
		details := &ArenaTeamDetails{
			RealmID:     realmID,
			ArenaTeamID: team.GetArenaTeamID(),
			Name:        team.GetName(),
			Type:        uint8(team.GetType()),
			Rating:      team.GetRating(),
			WeekGames:   team.GetWeekGames(),
			WeekWins:    team.GetWeekWins(),
			SeasonGames: team.GetSeasonGames(),
			SeasonWins:  team.GetSeasonWins(),
			Rank:        team.GetRank(),
			Members:     make([]ArenaTeamDetailsMember, 0, len(team.GetMembers())),
		}
		for _, member := range team.GetMembers() {
			details.Members = append(details.Members, ArenaTeamDetailsMember{
				PlayerGUID:       member.GetPlayerGUID(),
				PersonalRating:   member.GetPersonalRating(),
				WeekGames:        member.GetWeekGames(),
				WeekWins:         member.GetWeekWins(),
				SeasonGames:      member.GetSeasonGames(),
				SeasonWins:       member.GetSeasonWins(),
				MatchmakerRating: member.GetMatchmakerRating(),
				MaxMMR:           member.GetMaxMMR(),
			})
		}
		return details, nil
	case pbChar.GetArenaTeamResponse_NotFound:
		return nil, ErrArenaTeamNotFound
	default:
		return nil, fmt.Errorf("characters service failed arena team lookup with status %s", resp.GetStatus().String())
	}
}

func (r *charserverArenaTeamRepo) SaveStats(ctx context.Context, request ArenaTeamSaveStatsRequest) error {
	if r.client == nil {
		return errors.New("characters service client is nil")
	}

	members := make([]*pbChar.ArenaTeamStatsMember, 0, len(request.Members))
	for _, member := range request.Members {
		members = append(members, &pbChar.ArenaTeamStatsMember{
			PlayerGUID:       member.PlayerGUID,
			PersonalRating:   member.PersonalRating,
			WeekGames:        member.WeekGames,
			WeekWins:         member.WeekWins,
			SeasonGames:      member.SeasonGames,
			SeasonWins:       member.SeasonWins,
			MatchmakerRating: member.MatchmakerRating,
			MaxMMR:           member.MaxMMR,
			SaveArenaStats:   member.SaveArenaStats,
		})
	}

	resp, err := r.client.SaveArenaTeamStats(ctx, &pbChar.SaveArenaTeamStatsRequest{
		Api:         charserver.Ver,
		RealmID:     request.RealmID,
		ArenaTeamID: request.ArenaTeamID,
		Rating:      request.Rating,
		WeekGames:   request.WeekGames,
		WeekWins:    request.WeekWins,
		SeasonGames: request.SeasonGames,
		SeasonWins:  request.SeasonWins,
		Rank:        request.Rank,
		Slot:        request.Slot,
		Members:     members,
	})
	if err != nil {
		return err
	}
	if resp == nil {
		return errors.New("characters service returned nil arena team stats response")
	}

	switch resp.GetStatus() {
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK:
		return nil
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_NOT_FOUND:
		return ErrArenaTeamNotFound
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_MEMBER_MISMATCH:
		return ErrArenaTeamMemberMismatch
	case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_INVALID_TYPE:
		return ErrArenaTeamInvalidType
	default:
		return fmt.Errorf("characters service failed arena team stats save with status %s", resp.GetStatus().String())
	}
}
