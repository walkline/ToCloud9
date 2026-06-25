#ifndef __GUILD_API__
#define __GUILD_API__

#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

typedef enum GuildCreateErrorCode {
    GuildCreateErrorCodeNoError        = 0,
    GuildCreateErrorCodeNoHandler      = 1,
    GuildCreateErrorCodeNameExists     = 2,
    GuildCreateErrorCodeInvalidName    = 3,
    GuildCreateErrorCodeLeaderNotFound = 4,
    GuildCreateErrorCodeInternalError  = 5,
} GuildCreateErrorCode;

typedef struct {
    uint64_t leaderGuid;
    const char* guildName;
} GuildCreateRequest;

typedef struct {
    int errorCode;
    uint64_t guildId;
} GuildCreateResponse;

typedef GuildCreateResponse (*GuildCreateHandler)(GuildCreateRequest* request);

void SetGuildCreateHandler(GuildCreateHandler h);
GuildCreateResponse CallGuildCreateHandler(GuildCreateRequest* request);

#endif