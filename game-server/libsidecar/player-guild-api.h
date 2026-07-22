#ifndef __PLAYER_GUILD_API__
#define __PLAYER_GUILD_API__

#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

typedef enum PlayerGuildErrorCode {
    PlayerGuildErrorCodeNoError   = 0,
    PlayerGuildErrorCodeNoHandler = 1,
} PlayerGuildErrorCode;

// SetPlayerGuildFields request.
typedef struct {
    int errorCode;
    bool applied;
} SetPlayerGuildFieldsResponse;

typedef SetPlayerGuildFieldsResponse (*SetPlayerGuildFieldsHandler) (uint64_t /*player_guid*/, uint32_t /*guild_id*/, uint32_t /*rank*/);
void SetSetPlayerGuildFieldsHandler(SetPlayerGuildFieldsHandler h);
SetPlayerGuildFieldsResponse CallSetPlayerGuildFieldsHandler(uint64_t player_guid, uint32_t guild_id, uint32_t rank);

#endif
