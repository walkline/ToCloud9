#ifndef __LFG_API__
#define __LFG_API__

#include <stdint.h>
#include <stdlib.h>

typedef enum LfgPlayerLockInfoErrorCode {
    LfgPlayerLockInfoErrorCodeSuccess = 0,
    LfgPlayerLockInfoErrorCodeNoHandler = 1,
    LfgPlayerLockInfoErrorCodePlayerNotFound = 2,
    LfgPlayerLockInfoErrorCodeInternalError = 3,
} LfgPlayerLockInfoErrorCode;

typedef struct {
    uint32_t dungeonEntry;
    uint32_t lockStatus;
} LfgDungeonLock;

typedef struct {
    uint64_t playerGuid;
    uint32_t* dungeonEntries;
    int dungeonEntriesSize;
} LfgPlayerLockInfoRequest;

typedef struct {
    int errorCode;
    LfgDungeonLock* locks;
    int locksSize;
    uint32_t joinResult;
    uint32_t* validDungeonEntries;
    int validDungeonEntriesSize;
} LfgPlayerLockInfoResponse;

typedef enum LfgPlayerInfoErrorCode {
    LfgPlayerInfoErrorCodeSuccess = 0,
    LfgPlayerInfoErrorCodeNoHandler = 1,
    LfgPlayerInfoErrorCodePlayerNotFound = 2,
    LfgPlayerInfoErrorCodeInternalError = 3,
} LfgPlayerInfoErrorCode;

typedef struct {
    uint32_t itemId;
    uint32_t displayId;
    uint32_t count;
} LfgRewardItem;

typedef struct {
    uint32_t dungeonEntry;
    uint8_t done;
    uint32_t rewardMoney;
    uint32_t rewardXP;
    uint32_t rewardUnknown1;
    uint32_t rewardUnknown2;
    LfgRewardItem* rewardItems;
    int rewardItemsSize;
} LfgRandomDungeonInfo;

typedef struct {
    uint64_t playerGuid;
} LfgPlayerInfoRequest;

typedef struct {
    int errorCode;
    LfgRandomDungeonInfo* randomDungeons;
    int randomDungeonsSize;
    LfgDungeonLock* locks;
    int locksSize;
} LfgPlayerInfoResponse;

typedef enum LfgDungeonInfoErrorCode {
    LfgDungeonInfoErrorCodeSuccess = 0,
    LfgDungeonInfoErrorCodeNoHandler = 1,
    LfgDungeonInfoErrorCodeDungeonNotFound = 2,
    LfgDungeonInfoErrorCodeInternalError = 3,
} LfgDungeonInfoErrorCode;

typedef struct {
    uint32_t dungeonEntry;
} LfgDungeonInfoRequest;

typedef struct {
    int errorCode;
    uint32_t dungeonEntry;
    uint32_t dungeonId;
    uint32_t mapId;
    uint32_t typeId;
    uint32_t difficulty;
} LfgDungeonInfoResponse;

typedef enum LfgTeleportPlayerErrorCode {
    LfgTeleportPlayerErrorCodeSuccess = 0,
    LfgTeleportPlayerErrorCodeNoHandler = 1,
    LfgTeleportPlayerErrorCodePlayerNotFound = 2,
    LfgTeleportPlayerErrorCodeInternalError = 3,
} LfgTeleportPlayerErrorCode;

typedef struct {
    uint64_t playerGuid;
    uint8_t out;
    uint32_t dungeonEntry;
} LfgTeleportPlayerRequest;

typedef enum LfgBootVoteErrorCode {
    LfgBootVoteErrorCodeSuccess = 0,
    LfgBootVoteErrorCodeNoHandler = 1,
    LfgBootVoteErrorCodePlayerNotFound = 2,
    LfgBootVoteErrorCodeInternalError = 3,
} LfgBootVoteErrorCode;

typedef struct {
    uint64_t playerGuid;
    uint8_t agree;
} LfgBootVoteRequest;

typedef enum LfgMaterializeProposalErrorCode {
    LfgMaterializeProposalErrorCodeSuccess = 0,
    LfgMaterializeProposalErrorCodeNoHandler = 1,
    LfgMaterializeProposalErrorCodeDungeonNotFound = 2,
    LfgMaterializeProposalErrorCodeNoLocalPlayer = 3,
    LfgMaterializeProposalErrorCodeInternalError = 4,
} LfgMaterializeProposalErrorCode;

typedef struct {
    uint64_t playerGuid;
    uint8_t selectedRoles;
    uint8_t assignedRole;
    uint64_t queueLeaderGuid;
} LfgMaterializeProposalMember;

typedef struct {
    uint32_t realmId;
    uint32_t proposalId;
    uint32_t dungeonEntry;
    uint64_t leaderGuid;
    LfgMaterializeProposalMember* members;
    int membersSize;
} LfgMaterializeProposalRequest;

typedef LfgPlayerLockInfoResponse (*LfgPlayerLockInfoHandler)(LfgPlayerLockInfoRequest* request);
void SetLfgPlayerLockInfoHandler(LfgPlayerLockInfoHandler h);
LfgPlayerLockInfoResponse CallLfgPlayerLockInfoHandler(LfgPlayerLockInfoRequest* request);

typedef LfgPlayerInfoResponse (*LfgPlayerInfoHandler)(LfgPlayerInfoRequest* request);
void SetLfgPlayerInfoHandler(LfgPlayerInfoHandler h);
LfgPlayerInfoResponse CallLfgPlayerInfoHandler(LfgPlayerInfoRequest* request);

typedef LfgDungeonInfoResponse (*LfgDungeonInfoHandler)(LfgDungeonInfoRequest* request);
void SetLfgDungeonInfoHandler(LfgDungeonInfoHandler h);
LfgDungeonInfoResponse CallLfgDungeonInfoHandler(LfgDungeonInfoRequest* request);

typedef LfgTeleportPlayerErrorCode (*LfgTeleportPlayerHandler)(LfgTeleportPlayerRequest* request);
void SetLfgTeleportPlayerHandler(LfgTeleportPlayerHandler h);
LfgTeleportPlayerErrorCode CallLfgTeleportPlayerHandler(LfgTeleportPlayerRequest* request);

typedef LfgBootVoteErrorCode (*LfgBootVoteHandler)(LfgBootVoteRequest* request);
void SetLfgBootVoteHandler(LfgBootVoteHandler h);
LfgBootVoteErrorCode CallLfgBootVoteHandler(LfgBootVoteRequest* request);

typedef LfgMaterializeProposalErrorCode (*LfgMaterializeProposalHandler)(LfgMaterializeProposalRequest* request);
void SetLfgMaterializeProposalHandler(LfgMaterializeProposalHandler h);
LfgMaterializeProposalErrorCode CallLfgMaterializeProposalHandler(LfgMaterializeProposalRequest* request);

#endif
