#ifndef __PETITION_API__
#define __PETITION_API__

#include <stdint.h>
#include <stdlib.h>

#ifdef __cplusplus
extern "C" {
#endif

enum GuildPetitionCheckStatus {
    GuildPetitionCheckStatusOk                 = 0,
    GuildPetitionCheckStatusNoHandler          = 1,
    GuildPetitionCheckStatusPlayerNotFound     = 2,
    GuildPetitionCheckStatusPetitionNotFound   = 3,
    GuildPetitionCheckStatusNotPetitionOwner   = 4,
    GuildPetitionCheckStatusNotGuildPetition   = 5,
    GuildPetitionCheckStatusAlreadyInGuild     = 6,
    GuildPetitionCheckStatusNeedMoreSignatures = 7,
};

typedef struct {
    int status;
    /* Allocated with malloc by the handler, freed by the caller. */
    char* guildName;
    /* Allocated with malloc by the handler, freed by the caller. */
    uint64_t* signatoryGUIDs;
    int signatoryGUIDsSize;
} GuildPetitionValidationResult;

typedef GuildPetitionValidationResult (*CanTurnInGuildPetitionHandler) (uint64_t /*player_guid*/, uint64_t /*petition_item_guid*/);
void SetCanTurnInGuildPetitionHandler(CanTurnInGuildPetitionHandler h);
GuildPetitionValidationResult CallCanTurnInGuildPetitionHandler(uint64_t player_guid, uint64_t petition_item_guid);

#ifdef __cplusplus
}
#endif

#endif
