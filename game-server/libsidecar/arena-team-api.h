#ifndef __ARENA_TEAM_API__
#define __ARENA_TEAM_API__

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef enum ArenaTeamMutationStatus {
    ArenaTeamMutationStatusOk             = 0,
    ArenaTeamMutationStatusNotFound       = 1,
    ArenaTeamMutationStatusMemberMismatch = 2,
    ArenaTeamMutationStatusInvalidType    = 3,
    ArenaTeamMutationStatusNameExists     = 4,
    ArenaTeamMutationStatusAlreadyInTeam  = 5,
    ArenaTeamMutationStatusRosterFull     = 6,
    ArenaTeamMutationStatusFailed         = 7,
    ArenaTeamMutationStatusInvalidName    = 8,
} ArenaTeamMutationStatus;

typedef struct {
    uint8_t team;
    uint64_t playerGuid;
} TC9RatedArenaParticipant;

typedef struct {
    uint8_t team;
    uint64_t playerGuid;
    uint32_t personalRating;
    uint32_t weekGames;
    uint32_t seasonGames;
    uint32_t weekWins;
    uint32_t seasonWins;
    uint32_t matchmakerRating;
} TC9RatedArenaMemberResult;

typedef struct {
    uint32_t realmID;
    uint32_t arenaTeamID;
    char* teamName;
    int32_t ratingChange;
    uint32_t matchmakerRating;
} TC9RatedArenaTeamScore;

typedef struct {
    uint32_t status;
    TC9RatedArenaTeamScore allianceScore;
    TC9RatedArenaTeamScore hordeScore;
    TC9RatedArenaMemberResult* members;
    uint32_t membersSize;
} TC9FinishRatedArenaMatchResponse;

extern TC9FinishRatedArenaMatchResponse TC9FinishRatedArenaMatch(uint32_t ownerRealmID, uint8_t isCrossRealm, uint32_t instanceID, uint32_t arenaType, uint8_t winnerTeam, uint8_t validArena, uint32_t allianceArenaTeamID, uint32_t hordeArenaTeamID, uint32_t allianceArenaMatchmakerRating, uint32_t hordeArenaMatchmakerRating, TC9RatedArenaParticipant* participants, uint32_t participantsSize);
extern void TC9FreeFinishRatedArenaMatchResponse(TC9FinishRatedArenaMatchResponse* response);

#ifdef __cplusplus
}
#endif

#endif
