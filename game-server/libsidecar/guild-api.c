#include "guild-api.h"

static GuildCreateHandler guildCreateHandler;

void SetGuildCreateHandler(GuildCreateHandler h) {
    guildCreateHandler = h;
}

GuildCreateResponse CallGuildCreateHandler(GuildCreateRequest* request) {
    if (guildCreateHandler == 0) {
        GuildCreateResponse resp = {0};
        resp.errorCode = GuildCreateErrorCodeNoHandler;
        return resp;
    }

    return guildCreateHandler(request);
}