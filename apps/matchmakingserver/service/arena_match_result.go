package service

import (
	"context"
	"math"

	"github.com/walkline/ToCloud9/apps/matchmakingserver/battleground"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/repo"
	wowarena "github.com/walkline/ToCloud9/shared/wow/arena"
	wowguid "github.com/walkline/ToCloud9/shared/wow/guid"
)

const arenaTimelimitPointsLoss int32 = -16

type RatedArenaParticipant struct {
	Team       battleground.PVPTeam
	PlayerGUID uint64
}

type RatedArenaMatchResultRequest struct {
	OwnerRealmID uint32
	IsCrossRealm bool
	InstanceID   uint32
	ArenaType    uint8
	WinnerTeam   battleground.PVPTeam
	ValidArena   bool

	AllianceArenaTeamID           uint32
	HordeArenaTeamID              uint32
	AllianceArenaMatchmakerRating uint32
	HordeArenaMatchmakerRating    uint32
	Participants                  []RatedArenaParticipant
}

type RatedArenaTeamScore struct {
	RealmID          uint32
	ArenaTeamID      uint32
	TeamName         string
	RatingChange     int32
	MatchmakerRating uint32
}

type RatedArenaMemberResult struct {
	Team             battleground.PVPTeam
	PlayerGUID       uint64
	PersonalRating   uint32
	WeekGames        uint32
	SeasonGames      uint32
	WeekWins         uint32
	SeasonWins       uint32
	MatchmakerRating uint32
}

type RatedArenaMatchResult struct {
	AllianceScore RatedArenaTeamScore
	HordeScore    RatedArenaTeamScore
	MemberResults []RatedArenaMemberResult
}

type ratedArenaTeamRef struct {
	realmID uint32
	teamID  uint32
}

type ratedArenaTeamState struct {
	ref            ratedArenaTeamRef
	ownerRealmID   uint32
	team           *repo.ArenaTeamDetails
	mmr            uint32
	score          RatedArenaTeamScore
	changedMembers map[uint64]struct{}
}

func (s *battleGroundService) FinishRatedArenaMatch(ctx context.Context, request RatedArenaMatchResultRequest) (*RatedArenaMatchResult, error) {
	if s.arenaTeamRepo == nil {
		return nil, ErrRatedArenaUnavailable
	}
	if request.ArenaType != 2 && request.ArenaType != 3 && request.ArenaType != 5 {
		return nil, ErrInvalidArenaType
	}

	allianceRef := arenaTeamRefFromStartID(request.OwnerRealmID, request.AllianceArenaTeamID)
	hordeRef := arenaTeamRefFromStartID(request.OwnerRealmID, request.HordeArenaTeamID)
	if allianceRef.realmID == 0 || allianceRef.teamID == 0 || hordeRef.realmID == 0 || hordeRef.teamID == 0 {
		return nil, repo.ErrArenaTeamNotFound
	}

	allianceTeam, err := s.arenaTeamRepo.GetTeam(ctx, allianceRef.realmID, allianceRef.teamID)
	if err != nil {
		return nil, err
	}
	hordeTeam, err := s.arenaTeamRepo.GetTeam(ctx, hordeRef.realmID, hordeRef.teamID)
	if err != nil {
		return nil, err
	}
	if allianceTeam.Type != request.ArenaType || hordeTeam.Type != request.ArenaType {
		return nil, ErrInvalidArenaType
	}

	alliance := ratedArenaTeamState{
		ref:            allianceRef,
		ownerRealmID:   ratedArenaOwnerRealmID(request),
		team:           allianceTeam,
		mmr:            arenaMMRFallback(request.AllianceArenaMatchmakerRating, s.arenaStartMMR),
		changedMembers: map[uint64]struct{}{},
		score: RatedArenaTeamScore{
			RealmID:          allianceRef.realmID,
			ArenaTeamID:      allianceRef.teamID,
			TeamName:         allianceTeam.Name,
			MatchmakerRating: arenaMMRFallback(request.AllianceArenaMatchmakerRating, s.arenaStartMMR),
		},
	}
	horde := ratedArenaTeamState{
		ref:            hordeRef,
		ownerRealmID:   ratedArenaOwnerRealmID(request),
		team:           hordeTeam,
		mmr:            arenaMMRFallback(request.HordeArenaMatchmakerRating, s.arenaStartMMR),
		changedMembers: map[uint64]struct{}{},
		score: RatedArenaTeamScore{
			RealmID:          hordeRef.realmID,
			ArenaTeamID:      hordeRef.teamID,
			TeamName:         hordeTeam.Name,
			MatchmakerRating: arenaMMRFallback(request.HordeArenaMatchmakerRating, s.arenaStartMMR),
		},
	}

	result := &RatedArenaMatchResult{
		AllianceScore: alliance.score,
		HordeScore:    horde.score,
	}

	switch request.WinnerTeam {
	case battleground.TeamAlliance:
		err = s.finishRatedArenaWinLoss(ctx, &alliance, &horde, request.ValidArena, request.Participants, result)
	case battleground.TeamHorde:
		err = s.finishRatedArenaWinLoss(ctx, &horde, &alliance, request.ValidArena, request.Participants, result)
	default:
		err = s.finishRatedArenaDraw(ctx, &alliance, &horde, request.ValidArena, request.Participants, result)
	}
	if err != nil {
		return nil, err
	}

	result.AllianceScore = alliance.score
	result.HordeScore = horde.score

	return result, nil
}

func (s *battleGroundService) finishRatedArenaWinLoss(
	ctx context.Context,
	winner *ratedArenaTeamState,
	loser *ratedArenaTeamState,
	validArena bool,
	participants []RatedArenaParticipant,
	result *RatedArenaMatchResult,
) error {
	var winnerRatingChange int32
	var winnerMMRChange int32
	if validArena {
		winnerMMRChange = s.getMatchmakerRatingMod(winner.mmr, loser.mmr, true)
		winnerRatingChange = s.getRatingMod(winner.team.Rating, loser.mmr, true)
		applyArenaTeamFinish(&winner.team.Rating, &winner.team.WeekGames, &winner.team.SeasonGames, winnerRatingChange)
		winner.team.WeekWins++
		winner.team.SeasonWins++
	}

	loserMMRChange := s.getMatchmakerRatingMod(loser.mmr, winner.mmr, false)
	loserRatingChange := s.getRatingMod(loser.team.Rating, winner.mmr, false)
	applyArenaTeamFinish(&loser.team.Rating, &loser.team.WeekGames, &loser.team.SeasonGames, loserRatingChange)

	winner.score.RatingChange = winnerRatingChange
	winner.score.MatchmakerRating = winner.mmr
	loser.score.RatingChange = loserRatingChange
	loser.score.MatchmakerRating = loser.mmr

	winnerMembers, err := participantSet(participants, teamForState(winner, result), winner.ref.realmID)
	if err != nil {
		return err
	}
	loserMembers, err := participantSet(participants, teamForState(loser, result), loser.ref.realmID)
	if err != nil {
		return err
	}

	if validArena {
		if err = s.applyWinnerMembers(winner, loser.mmr, winnerMMRChange, winnerMembers, result); err != nil {
			return err
		}
	}
	if err = s.applyLoserMembers(loser, winner.mmr, loserMMRChange, loserMembers, result); err != nil {
		return err
	}

	if validArena {
		if err := s.saveArenaTeamState(ctx, winner); err != nil {
			return err
		}
	}
	return s.saveArenaTeamState(ctx, loser)
}

func (s *battleGroundService) finishRatedArenaDraw(
	ctx context.Context,
	alliance *ratedArenaTeamState,
	horde *ratedArenaTeamState,
	validArena bool,
	participants []RatedArenaParticipant,
	result *RatedArenaMatchResult,
) error {
	applyArenaTeamFinish(&alliance.team.Rating, &alliance.team.WeekGames, &alliance.team.SeasonGames, arenaTimelimitPointsLoss)
	applyArenaTeamFinish(&horde.team.Rating, &horde.team.WeekGames, &horde.team.SeasonGames, arenaTimelimitPointsLoss)
	alliance.score.RatingChange = arenaTimelimitPointsLoss
	horde.score.RatingChange = arenaTimelimitPointsLoss

	allianceMembers, err := participantSet(participants, battleground.TeamAlliance, alliance.ref.realmID)
	if err != nil {
		return err
	}
	hordeMembers, err := participantSet(participants, battleground.TeamHorde, horde.ref.realmID)
	if err != nil {
		return err
	}

	if err = s.applyLoserMembers(alliance, horde.mmr, 0, allianceMembers, result); err != nil {
		return err
	}
	if err = s.applyLoserMembers(horde, alliance.mmr, 0, hordeMembers, result); err != nil {
		return err
	}

	if validArena {
		if err := s.saveArenaTeamState(ctx, horde); err != nil {
			return err
		}
	}
	return s.saveArenaTeamState(ctx, alliance)
}

func (s *battleGroundService) applyWinnerMembers(team *ratedArenaTeamState, opponentMMR uint32, teamMMRChange int32, participants map[uint64]struct{}, result *RatedArenaMatchResult) error {
	for i := range team.team.Members {
		member := &team.team.Members[i]
		if _, ok := participants[member.PlayerGUID]; !ok {
			continue
		}
		mod := s.getRatingMod(member.PersonalRating, opponentMMR, true)
		member.PersonalRating = applyRatingChange(member.PersonalRating, mod)
		if member.MatchmakerRating == 0 {
			member.MatchmakerRating = s.arenaStartMMR
		}
		if member.MaxMMR == 0 {
			member.MaxMMR = member.MatchmakerRating
		}
		if member.MatchmakerRating < team.team.Rating {
			mmrChange := minInt32(teamMMRChange, int32(team.team.Rating-member.MatchmakerRating))
			member.MatchmakerRating, member.MaxMMR = applyMMRChange(member.MatchmakerRating, member.MaxMMR, mmrChange, s.arenaRatingConfig.MaxAllowedMMRDrop)
		}
		member.WeekGames++
		member.SeasonGames++
		member.WeekWins++
		member.SeasonWins++
		team.changedMembers[member.PlayerGUID] = struct{}{}
		result.MemberResults = append(result.MemberResults, arenaMemberResult(teamForState(team, result), team.ownerRealmID, team.ref.realmID, member))
	}
	return ensureParticipantsMatched(participants, team.team)
}

func (s *battleGroundService) applyLoserMembers(team *ratedArenaTeamState, opponentMMR uint32, teamMMRChange int32, participants map[uint64]struct{}, result *RatedArenaMatchResult) error {
	for i := range team.team.Members {
		member := &team.team.Members[i]
		if _, ok := participants[member.PlayerGUID]; !ok {
			continue
		}
		mod := s.getRatingMod(member.PersonalRating, opponentMMR, false)
		member.PersonalRating = applyRatingChange(member.PersonalRating, mod)
		if member.MatchmakerRating == 0 {
			member.MatchmakerRating = s.arenaStartMMR
		}
		if member.MaxMMR == 0 {
			member.MaxMMR = member.MatchmakerRating
		}
		member.MatchmakerRating, member.MaxMMR = applyMMRChange(member.MatchmakerRating, member.MaxMMR, teamMMRChange, s.arenaRatingConfig.MaxAllowedMMRDrop)
		member.WeekGames++
		member.SeasonGames++
		team.changedMembers[member.PlayerGUID] = struct{}{}
		result.MemberResults = append(result.MemberResults, arenaMemberResult(teamForState(team, result), team.ownerRealmID, team.ref.realmID, member))
	}
	return ensureParticipantsMatched(participants, team.team)
}

func (s *battleGroundService) saveArenaTeamState(ctx context.Context, team *ratedArenaTeamState) error {
	members := make([]repo.ArenaTeamSaveStatsMember, 0, len(team.team.Members))
	slot, ok := arenaSlotByType(team.team.Type)
	if !ok {
		return ErrInvalidArenaType
	}
	for _, member := range team.team.Members {
		if _, ok := team.changedMembers[member.PlayerGUID]; !ok {
			continue
		}
		members = append(members, repo.ArenaTeamSaveStatsMember{
			PlayerGUID:       member.PlayerGUID,
			PersonalRating:   member.PersonalRating,
			WeekGames:        member.WeekGames,
			WeekWins:         member.WeekWins,
			SeasonGames:      member.SeasonGames,
			SeasonWins:       member.SeasonWins,
			MatchmakerRating: member.MatchmakerRating,
			MaxMMR:           member.MaxMMR,
			SaveArenaStats:   true,
		})
	}
	return s.arenaTeamRepo.SaveStats(ctx, repo.ArenaTeamSaveStatsRequest{
		RealmID:     team.ref.realmID,
		ArenaTeamID: team.ref.teamID,
		Rating:      team.team.Rating,
		WeekGames:   team.team.WeekGames,
		WeekWins:    team.team.WeekWins,
		SeasonGames: team.team.SeasonGames,
		SeasonWins:  team.team.SeasonWins,
		Rank:        team.team.Rank,
		Slot:        uint32(slot),
		Members:     members,
	})
}

func (s *battleGroundService) getChanceAgainst(ownRating, opponentRating uint32) float64 {
	return 1.0 / (1.0 + math.Exp(math.Log(10.0)*(float64(opponentRating)-float64(ownRating))/650.0))
}

func (s *battleGroundService) getMatchmakerRatingMod(ownRating, opponentRating uint32, won bool) int32 {
	chance := s.getChanceAgainst(ownRating, opponentRating)
	wonMod := 0.0
	if won {
		wonMod = 1.0
	}
	return int32(math.Ceil((wonMod - chance) * s.arenaRatingConfig.MatchmakerModifier))
}

func (s *battleGroundService) getRatingMod(ownRating, opponentRating uint32, won bool) int32 {
	chance := s.getChanceAgainst(ownRating, opponentRating)
	var mod float64
	if won {
		if ownRating < 1300 {
			if ownRating < 1000 {
				mod = s.arenaRatingConfig.WinModifier1 * (1.0 - chance)
			} else {
				mod = ((s.arenaRatingConfig.WinModifier1 / 2.0) + ((s.arenaRatingConfig.WinModifier1 / 2.0) * (1300.0 - float64(ownRating)) / 300.0)) * (1.0 - chance)
			}
		} else {
			mod = s.arenaRatingConfig.WinModifier2 * (1.0 - chance)
		}
	} else {
		mod = s.arenaRatingConfig.LoseModifier * (-chance)
	}
	return int32(math.Ceil(mod))
}

func arenaTeamRefFromStartID(ownerRealmID uint32, startTeamID uint32) ratedArenaTeamRef {
	realmID := wowarena.TeamIDRealmID(startTeamID)
	teamID := wowarena.TeamIDCounter(startTeamID)
	if realmID == 0 {
		realmID = ownerRealmID
	}
	return ratedArenaTeamRef{realmID: realmID, teamID: teamID}
}

func ratedArenaOwnerRealmID(request RatedArenaMatchResultRequest) uint32 {
	if request.IsCrossRealm {
		return 0
	}
	return request.OwnerRealmID
}

func arenaMMRFallback(value, fallback uint32) uint32 {
	if value == 0 {
		return fallback
	}
	return value
}

func arenaSlotByType(arenaType uint8) (uint8, bool) {
	switch arenaType {
	case 2:
		return 0, true
	case 3:
		return 1, true
	case 5:
		return 2, true
	default:
		return 0, false
	}
}

func applyArenaTeamFinish(rating *uint32, weekGames *uint32, seasonGames *uint32, mod int32) {
	*rating = applyRatingChange(*rating, mod)
	*weekGames = *weekGames + 1
	*seasonGames = *seasonGames + 1
}

func applyRatingChange(rating uint32, mod int32) uint32 {
	if int64(rating)+int64(mod) < 0 {
		return 0
	}
	return uint32(int64(rating) + int64(mod))
}

func applyMMRChange(mmr, maxMMR uint32, mod int32, maxAllowedDrop uint32) (uint32, uint32) {
	if mod < 0 {
		maxDropWindow := int32(mmr) - int32(maxMMR) + int32(maxAllowedDrop)
		mod = minInt32(maxInt32(-maxDropWindow, mod), 0)
	}
	next := applyRatingChange(mmr, mod)
	if next > maxMMR {
		maxMMR = next
	}
	return next, maxMMR
}

func participantSet(participants []RatedArenaParticipant, team battleground.PVPTeam, teamRealmID uint32) (map[uint64]struct{}, error) {
	set := make(map[uint64]struct{})
	for _, participant := range participants {
		if participant.Team != team {
			continue
		}
		participantRealmID := wowguid.PlayerRealmIDOrDefault(teamRealmID, participant.PlayerGUID)
		if participantRealmID != teamRealmID {
			return nil, repo.ErrArenaTeamMemberMismatch
		}
		set[wowguid.PlayerLowGUID(participant.PlayerGUID)] = struct{}{}
	}
	return set, nil
}

func ensureParticipantsMatched(participants map[uint64]struct{}, team *repo.ArenaTeamDetails) error {
	for _, member := range team.Members {
		delete(participants, member.PlayerGUID)
	}
	if len(participants) > 0 {
		return repo.ErrArenaTeamMemberMismatch
	}
	return nil
}

func teamForState(team *ratedArenaTeamState, result *RatedArenaMatchResult) battleground.PVPTeam {
	if team.score.RealmID == result.AllianceScore.RealmID && team.score.ArenaTeamID == result.AllianceScore.ArenaTeamID {
		return battleground.TeamAlliance
	}
	return battleground.TeamHorde
}

func arenaMemberResult(team battleground.PVPTeam, ownerRealmID uint32, memberRealmID uint32, member *repo.ArenaTeamDetailsMember) RatedArenaMemberResult {
	return RatedArenaMemberResult{
		Team:             team,
		PlayerGUID:       wowguid.PlayerGUIDForRealm(ownerRealmID, memberRealmID, member.PlayerGUID),
		PersonalRating:   member.PersonalRating,
		WeekGames:        member.WeekGames,
		SeasonGames:      member.SeasonGames,
		WeekWins:         member.WeekWins,
		SeasonWins:       member.SeasonWins,
		MatchmakerRating: member.MatchmakerRating,
	}
}

func minInt32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
