#include "lfg-api.h"

static LfgPlayerLockInfoHandler lfgPlayerLockInfoHandler = 0;
static LfgPlayerInfoHandler lfgPlayerInfoHandler = 0;
static LfgDungeonInfoHandler lfgDungeonInfoHandler = 0;
static LfgTeleportPlayerHandler lfgTeleportPlayerHandler = 0;
static LfgBootVoteHandler lfgBootVoteHandler = 0;
static LfgMaterializeProposalHandler lfgMaterializeProposalHandler = 0;

void SetLfgPlayerLockInfoHandler(LfgPlayerLockInfoHandler h) {
    lfgPlayerLockInfoHandler = h;
}

LfgPlayerLockInfoResponse CallLfgPlayerLockInfoHandler(LfgPlayerLockInfoRequest* request) {
    if (lfgPlayerLockInfoHandler == 0) {
        LfgPlayerLockInfoResponse resp;
        resp.errorCode = LfgPlayerLockInfoErrorCodeNoHandler;
        resp.locks = 0;
        resp.locksSize = 0;
        resp.joinResult = 0;
        resp.validDungeonEntries = 0;
        resp.validDungeonEntriesSize = 0;
        return resp;
    }

    return lfgPlayerLockInfoHandler(request);
}

void SetLfgPlayerInfoHandler(LfgPlayerInfoHandler h) {
    lfgPlayerInfoHandler = h;
}

LfgPlayerInfoResponse CallLfgPlayerInfoHandler(LfgPlayerInfoRequest* request) {
    if (lfgPlayerInfoHandler == 0) {
        LfgPlayerInfoResponse resp;
        resp.errorCode = LfgPlayerInfoErrorCodeNoHandler;
        resp.randomDungeons = 0;
        resp.randomDungeonsSize = 0;
        resp.locks = 0;
        resp.locksSize = 0;
        return resp;
    }

    return lfgPlayerInfoHandler(request);
}

void SetLfgDungeonInfoHandler(LfgDungeonInfoHandler h) {
    lfgDungeonInfoHandler = h;
}

LfgDungeonInfoResponse CallLfgDungeonInfoHandler(LfgDungeonInfoRequest* request) {
    if (lfgDungeonInfoHandler == 0) {
        LfgDungeonInfoResponse resp;
        resp.errorCode = LfgDungeonInfoErrorCodeNoHandler;
        resp.dungeonEntry = 0;
        resp.dungeonId = 0;
        resp.mapId = 0;
        resp.typeId = 0;
        resp.difficulty = 0;
        return resp;
    }

    return lfgDungeonInfoHandler(request);
}

void SetLfgTeleportPlayerHandler(LfgTeleportPlayerHandler h) {
    lfgTeleportPlayerHandler = h;
}

LfgTeleportPlayerErrorCode CallLfgTeleportPlayerHandler(LfgTeleportPlayerRequest* request) {
    if (lfgTeleportPlayerHandler == 0) {
        return LfgTeleportPlayerErrorCodeNoHandler;
    }

    return lfgTeleportPlayerHandler(request);
}

void SetLfgBootVoteHandler(LfgBootVoteHandler h) {
    lfgBootVoteHandler = h;
}

LfgBootVoteErrorCode CallLfgBootVoteHandler(LfgBootVoteRequest* request) {
    if (lfgBootVoteHandler == 0) {
        return LfgBootVoteErrorCodeNoHandler;
    }

    return lfgBootVoteHandler(request);
}

void SetLfgMaterializeProposalHandler(LfgMaterializeProposalHandler h) {
    lfgMaterializeProposalHandler = h;
}

LfgMaterializeProposalErrorCode CallLfgMaterializeProposalHandler(LfgMaterializeProposalRequest* request) {
    if (lfgMaterializeProposalHandler == 0) {
        return LfgMaterializeProposalErrorCodeNoHandler;
    }

    return lfgMaterializeProposalHandler(request);
}
