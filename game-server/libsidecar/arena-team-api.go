package main

/*
#include "arena-team-api.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"errors"
	"time"
	"unsafe"

	"github.com/rs/zerolog/log"

	mmPb "github.com/walkline/ToCloud9/gen/matchmaking/pb"
)

// TC9FinishRatedArenaMatch sends a rated arena outcome to matchmaking for rating/stat calculation and persistence.
//
//export TC9FinishRatedArenaMatch
func TC9FinishRatedArenaMatch(
	ownerRealmID C.uint32_t,
	isCrossRealm C.uint8_t,
	instanceID C.uint32_t,
	arenaType C.uint32_t,
	winnerTeam C.uint8_t,
	validArena C.uint8_t,
	allianceArenaTeamID C.uint32_t,
	hordeArenaTeamID C.uint32_t,
	allianceArenaMatchmakerRating C.uint32_t,
	hordeArenaMatchmakerRating C.uint32_t,
	participants *C.TC9RatedArenaParticipant,
	participantsSize C.uint32_t,
) C.TC9FinishRatedArenaMatchResponse {
	if matchmakingServiceClient == nil {
		return finishRatedArenaMatchResponse(C.ArenaTeamMutationStatusFailed, nil)
	}

	log.Debug().
		Uint32("ownerRealmID", uint32(ownerRealmID)).
		Bool("isCrossrealm", isCrossRealm != 0).
		Uint32("instanceID", uint32(instanceID)).
		Uint32("arenaType", uint32(arenaType)).
		Uint8("winnerTeam", uint8(winnerTeam)).
		Bool("validArena", validArena != 0).
		Uint32("allianceArenaTeamID", uint32(allianceArenaTeamID)).
		Uint32("hordeArenaTeamID", uint32(hordeArenaTeamID)).
		Uint32("allianceArenaMMR", uint32(allianceArenaMatchmakerRating)).
		Uint32("hordeArenaMMR", uint32(hordeArenaMatchmakerRating)).
		Uint32("participants", uint32(participantsSize)).
		Msg("TC9 finish rated arena match request")

	protoParticipants := make([]*mmPb.RatedArenaParticipant, 0, uint32(participantsSize))
	if participantsSize > 0 {
		participantSlice := unsafe.Slice((*C.TC9RatedArenaParticipant)(unsafe.Pointer(participants)), int(participantsSize))
		for _, participant := range participantSlice {
			protoParticipants = append(protoParticipants, &mmPb.RatedArenaParticipant{
				Team:       pvpTeamToProto(uint8(participant.team)),
				PlayerGUID: uint64(participant.playerGuid),
			})
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := matchmakingServiceClient.FinishRatedArenaMatch(ctx, &mmPb.FinishRatedArenaMatchRequest{
		Api:                           matchmakingSupportedVer,
		OwnerRealmID:                  uint32(ownerRealmID),
		IsCrossRealm:                  isCrossRealm != 0,
		InstanceID:                    uint32(instanceID),
		ArenaType:                     uint32(arenaType),
		WinnerTeam:                    pvpTeamToProto(uint8(winnerTeam)),
		ValidArena:                    validArena != 0,
		AllianceArenaTeamID:           uint32(allianceArenaTeamID),
		HordeArenaTeamID:              uint32(hordeArenaTeamID),
		AllianceArenaMatchmakerRating: uint32(allianceArenaMatchmakerRating),
		HordeArenaMatchmakerRating:    uint32(hordeArenaMatchmakerRating),
		Participants:                  protoParticipants,
	})
	if err != nil {
		logArenaTeamMutationError(err, "finish rated arena match", ownerRealmID, 0)
		return finishRatedArenaMatchResponse(C.ArenaTeamMutationStatusFailed, nil)
	}
	if resp == nil {
		logArenaTeamMutationError(errors.New("matchmaking service returned nil rated arena result response"), "finish rated arena match", ownerRealmID, 0)
		return finishRatedArenaMatchResponse(C.ArenaTeamMutationStatusFailed, nil)
	}

	log.Debug().
		Uint32("ownerRealmID", uint32(ownerRealmID)).
		Uint32("instanceID", uint32(instanceID)).
		Str("status", resp.GetStatus().String()).
		Uint32("memberResults", uint32(len(resp.GetMemberResults()))).
		Msg("TC9 finish rated arena match response")

	return finishRatedArenaMatchResponse(matchmakingArenaTeamMutationStatusToC(resp.GetStatus()), resp)
}

//export TC9FreeFinishRatedArenaMatchResponse
func TC9FreeFinishRatedArenaMatchResponse(response *C.TC9FinishRatedArenaMatchResponse) {
	if response == nil {
		return
	}
	if response.allianceScore.teamName != nil {
		C.free(unsafe.Pointer(response.allianceScore.teamName))
		response.allianceScore.teamName = nil
	}
	if response.hordeScore.teamName != nil {
		C.free(unsafe.Pointer(response.hordeScore.teamName))
		response.hordeScore.teamName = nil
	}
	if response.members != nil {
		C.free(unsafe.Pointer(response.members))
		response.members = nil
	}
	response.membersSize = 0
}

func matchmakingArenaTeamMutationStatusToC(status mmPb.MatchmakingArenaTeamMutationStatus) C.ArenaTeamMutationStatus {
	switch status {
	case mmPb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_OK:
		return C.ArenaTeamMutationStatusOk
	case mmPb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_NOT_FOUND:
		return C.ArenaTeamMutationStatusNotFound
	case mmPb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_MEMBER_MISMATCH:
		return C.ArenaTeamMutationStatusMemberMismatch
	case mmPb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_INVALID_TYPE:
		return C.ArenaTeamMutationStatusInvalidType
	case mmPb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_NAME_EXISTS:
		return C.ArenaTeamMutationStatusNameExists
	case mmPb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_ALREADY_IN_TEAM:
		return C.ArenaTeamMutationStatusAlreadyInTeam
	case mmPb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_ROSTER_FULL:
		return C.ArenaTeamMutationStatusRosterFull
	case mmPb.MatchmakingArenaTeamMutationStatus_MATCHMAKING_ARENA_TEAM_MUTATION_INVALID_NAME:
		return C.ArenaTeamMutationStatusInvalidName
	default:
		return C.ArenaTeamMutationStatusFailed
	}
}

func finishRatedArenaMatchResponse(status C.ArenaTeamMutationStatus, resp *mmPb.FinishRatedArenaMatchResponse) C.TC9FinishRatedArenaMatchResponse {
	result := C.TC9FinishRatedArenaMatchResponse{
		status: C.uint32_t(status),
	}
	if resp == nil {
		return result
	}

	result.allianceScore = ratedArenaTeamScoreToC(resp.GetAllianceScore())
	result.hordeScore = ratedArenaTeamScoreToC(resp.GetHordeScore())

	members := resp.GetMemberResults()
	if len(members) > 0 {
		result.members = (*C.TC9RatedArenaMemberResult)(C.malloc(C.size_t(len(members)) * C.size_t(unsafe.Sizeof(C.TC9RatedArenaMemberResult{}))))
		if result.members == nil {
			result.status = C.uint32_t(C.ArenaTeamMutationStatusFailed)
			return result
		}
		memberSlice := unsafe.Slice((*C.TC9RatedArenaMemberResult)(unsafe.Pointer(result.members)), len(members))
		for i, member := range members {
			memberSlice[i] = C.TC9RatedArenaMemberResult{
				team:             C.uint8_t(protoPVPTeamToC(member.GetTeam())),
				playerGuid:       C.uint64_t(member.GetPlayerGUID()),
				personalRating:   C.uint32_t(member.GetPersonalRating()),
				weekGames:        C.uint32_t(member.GetWeekGames()),
				seasonGames:      C.uint32_t(member.GetSeasonGames()),
				weekWins:         C.uint32_t(member.GetWeekWins()),
				seasonWins:       C.uint32_t(member.GetSeasonWins()),
				matchmakerRating: C.uint32_t(member.GetMatchmakerRating()),
			}
		}
		result.membersSize = C.uint32_t(len(members))
	}

	return result
}

func ratedArenaTeamScoreToC(score *mmPb.RatedArenaTeamScore) C.TC9RatedArenaTeamScore {
	if score == nil {
		return C.TC9RatedArenaTeamScore{}
	}
	return C.TC9RatedArenaTeamScore{
		realmID:          C.uint32_t(score.GetRealmID()),
		arenaTeamID:      C.uint32_t(score.GetArenaTeamID()),
		teamName:         C.CString(score.GetTeamName()),
		ratingChange:     C.int32_t(score.GetRatingChange()),
		matchmakerRating: C.uint32_t(score.GetMatchmakerRating()),
	}
}

func pvpTeamToProto(team uint8) mmPb.PVPTeamID {
	switch team {
	case 1:
		return mmPb.PVPTeamID_Alliance
	case 2:
		return mmPb.PVPTeamID_Horde
	default:
		return mmPb.PVPTeamID_Any
	}
}

func protoPVPTeamToC(team mmPb.PVPTeamID) uint8 {
	switch team {
	case mmPb.PVPTeamID_Alliance:
		return 1
	case mmPb.PVPTeamID_Horde:
		return 2
	default:
		return 0
	}
}

func logArenaTeamMutationError(err error, action string, realmID C.uint32_t, arenaTeamID C.uint32_t) {
	log.Error().
		Err(err).
		Str("action", action).
		Uint32("realmID", uint32(realmID)).
		Uint32("arenaTeamID", uint32(arenaTeamID)).
		Msg("arena team mutation failed")
}
