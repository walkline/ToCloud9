#include "petition-api.h"

static CanTurnInGuildPetitionHandler canTurnInGuildPetitionHandler;
void SetCanTurnInGuildPetitionHandler(CanTurnInGuildPetitionHandler h) {
    canTurnInGuildPetitionHandler = h;
}

GuildPetitionValidationResult CallCanTurnInGuildPetitionHandler(uint64_t player_guid, uint64_t petition_item_guid) {
    if (canTurnInGuildPetitionHandler == 0) {
        GuildPetitionValidationResult resp;
        resp.status = GuildPetitionCheckStatusNoHandler;
        resp.guildName = 0;
        resp.signatoryGUIDs = 0;
        resp.signatoryGUIDsSize = 0;
        return resp;
    }

    return canTurnInGuildPetitionHandler(player_guid, petition_item_guid);
}
