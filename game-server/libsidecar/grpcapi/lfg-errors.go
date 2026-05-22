package grpcapi

import "errors"

type LFGPlayerLockInfoErrorCode int

const (
	LFGPlayerLockInfoErrorCodeSuccess LFGPlayerLockInfoErrorCode = iota
	LFGPlayerLockInfoErrorCodeNoHandler
	LFGPlayerLockInfoErrorCodePlayerNotFound
	LFGPlayerLockInfoErrorCodeInternalError
)

type LFGPlayerLockInfoError LFGPlayerLockInfoErrorCode

func (e LFGPlayerLockInfoError) Error() string {
	switch LFGPlayerLockInfoErrorCode(e) {
	case LFGPlayerLockInfoErrorCodeNoHandler:
		return "lfg player lock info handler is not registered"
	case LFGPlayerLockInfoErrorCodePlayerNotFound:
		return "lfg player not found"
	default:
		return "lfg player lock info handler failed"
	}
}

func LFGPlayerLockInfoErrorCodeForError(err error) LFGPlayerLockInfoErrorCode {
	var lockInfoErr LFGPlayerLockInfoError
	if errors.As(err, &lockInfoErr) {
		return LFGPlayerLockInfoErrorCode(lockInfoErr)
	}
	return LFGPlayerLockInfoErrorCodeInternalError
}

type LFGPlayerInfoErrorCode int

const (
	LFGPlayerInfoErrorCodeSuccess LFGPlayerInfoErrorCode = iota
	LFGPlayerInfoErrorCodeNoHandler
	LFGPlayerInfoErrorCodePlayerNotFound
	LFGPlayerInfoErrorCodeInternalError
)

type LFGPlayerInfoError LFGPlayerInfoErrorCode

func (e LFGPlayerInfoError) Error() string {
	switch LFGPlayerInfoErrorCode(e) {
	case LFGPlayerInfoErrorCodeNoHandler:
		return "lfg player info handler is not registered"
	case LFGPlayerInfoErrorCodePlayerNotFound:
		return "lfg player not found"
	default:
		return "lfg player info handler failed"
	}
}

func LFGPlayerInfoErrorCodeForError(err error) LFGPlayerInfoErrorCode {
	var playerInfoErr LFGPlayerInfoError
	if errors.As(err, &playerInfoErr) {
		return LFGPlayerInfoErrorCode(playerInfoErr)
	}
	return LFGPlayerInfoErrorCodeInternalError
}

type LFGDungeonInfoErrorCode int

const (
	LFGDungeonInfoErrorCodeSuccess LFGDungeonInfoErrorCode = iota
	LFGDungeonInfoErrorCodeNoHandler
	LFGDungeonInfoErrorCodeDungeonNotFound
	LFGDungeonInfoErrorCodeInternalError
)

type LFGDungeonInfoError LFGDungeonInfoErrorCode

func (e LFGDungeonInfoError) Error() string {
	switch LFGDungeonInfoErrorCode(e) {
	case LFGDungeonInfoErrorCodeNoHandler:
		return "lfg dungeon info handler is not registered"
	case LFGDungeonInfoErrorCodeDungeonNotFound:
		return "lfg dungeon info dungeon not found"
	default:
		return "lfg dungeon info handler failed"
	}
}

func LFGDungeonInfoErrorCodeForError(err error) LFGDungeonInfoErrorCode {
	var dungeonInfoErr LFGDungeonInfoError
	if errors.As(err, &dungeonInfoErr) {
		return LFGDungeonInfoErrorCode(dungeonInfoErr)
	}
	return LFGDungeonInfoErrorCodeInternalError
}

type LFGTeleportPlayerErrorCode int

const (
	LFGTeleportPlayerErrorCodeSuccess LFGTeleportPlayerErrorCode = iota
	LFGTeleportPlayerErrorCodeNoHandler
	LFGTeleportPlayerErrorCodePlayerNotFound
	LFGTeleportPlayerErrorCodeInternalError
)

type LFGTeleportPlayerError LFGTeleportPlayerErrorCode

func (e LFGTeleportPlayerError) Error() string {
	switch LFGTeleportPlayerErrorCode(e) {
	case LFGTeleportPlayerErrorCodeNoHandler:
		return "lfg teleport player handler is not registered"
	case LFGTeleportPlayerErrorCodePlayerNotFound:
		return "lfg teleport player not found"
	default:
		return "lfg teleport player handler failed"
	}
}

func LFGTeleportPlayerErrorCodeForError(err error) LFGTeleportPlayerErrorCode {
	var teleportErr LFGTeleportPlayerError
	if errors.As(err, &teleportErr) {
		return LFGTeleportPlayerErrorCode(teleportErr)
	}
	return LFGTeleportPlayerErrorCodeInternalError
}

type LFGSetBootVoteErrorCode int

const (
	LFGSetBootVoteErrorCodeSuccess LFGSetBootVoteErrorCode = iota
	LFGSetBootVoteErrorCodeNoHandler
	LFGSetBootVoteErrorCodePlayerNotFound
	LFGSetBootVoteErrorCodeInternalError
)

type LFGSetBootVoteError LFGSetBootVoteErrorCode

func (e LFGSetBootVoteError) Error() string {
	switch LFGSetBootVoteErrorCode(e) {
	case LFGSetBootVoteErrorCodeNoHandler:
		return "lfg boot vote handler is not registered"
	case LFGSetBootVoteErrorCodePlayerNotFound:
		return "lfg boot vote player not found"
	default:
		return "lfg boot vote handler failed"
	}
}

func LFGSetBootVoteErrorCodeForError(err error) LFGSetBootVoteErrorCode {
	var bootVoteErr LFGSetBootVoteError
	if errors.As(err, &bootVoteErr) {
		return LFGSetBootVoteErrorCode(bootVoteErr)
	}
	return LFGSetBootVoteErrorCodeInternalError
}

type LFGMaterializeProposalErrorCode int

const (
	LFGMaterializeProposalErrorCodeSuccess LFGMaterializeProposalErrorCode = iota
	LFGMaterializeProposalErrorCodeNoHandler
	LFGMaterializeProposalErrorCodeDungeonNotFound
	LFGMaterializeProposalErrorCodeNoLocalPlayer
	LFGMaterializeProposalErrorCodeInternalError
)

type LFGMaterializeProposalError LFGMaterializeProposalErrorCode

func (e LFGMaterializeProposalError) Error() string {
	switch LFGMaterializeProposalErrorCode(e) {
	case LFGMaterializeProposalErrorCodeNoHandler:
		return "lfg materialize proposal handler is not registered"
	case LFGMaterializeProposalErrorCodeDungeonNotFound:
		return "lfg materialize proposal dungeon not found"
	case LFGMaterializeProposalErrorCodeNoLocalPlayer:
		return "lfg materialize proposal has no local player"
	default:
		return "lfg materialize proposal handler failed"
	}
}

func LFGMaterializeProposalErrorCodeForError(err error) LFGMaterializeProposalErrorCode {
	var materializeErr LFGMaterializeProposalError
	if errors.As(err, &materializeErr) {
		return LFGMaterializeProposalErrorCode(materializeErr)
	}
	return LFGMaterializeProposalErrorCodeInternalError
}
