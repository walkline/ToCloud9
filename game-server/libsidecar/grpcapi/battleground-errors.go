package grpcapi

import "fmt"

type BattlegroundError uint8

const (
	BattlegroundErrorOK BattlegroundError = iota
	BattlegroundErrorNoHandler
	BattlegroundErrorFailedToCreateBG
	BattlegroundErrorBattlegroundNotFound
)

func (e BattlegroundError) Error() string {
	switch e {
	case BattlegroundErrorOK:
		return "ok"
	case BattlegroundErrorNoHandler:
		return "no handler defined"
	case BattlegroundErrorFailedToCreateBG:
		return "failed to create bg"
	case BattlegroundErrorBattlegroundNotFound:
		return "battleground not found"
	default:
		return fmt.Sprintf("unk battlground error %d", e)
	}
}

type BattlegroundJoinCheckError uint8

const (
	BattlegroundJoinCheckErrorOK BattlegroundJoinCheckError = iota
	BattlegroundJoinCheckErrorNoHandler
	BattlegroundJoinCheckErrorResponseIsFalse
	BattlegroundJoinCheckErrorPlayerNotFound
)

func (e BattlegroundJoinCheckError) Error() string {
	switch e {
	case BattlegroundJoinCheckErrorOK:
		return "ok"
	case BattlegroundJoinCheckErrorNoHandler:
		return "no handler defined"
	case BattlegroundJoinCheckErrorResponseIsFalse:
		return "check returned false"
	case BattlegroundJoinCheckErrorPlayerNotFound:
		return "player not found"
	default:
		return fmt.Sprintf("unk battlground join error %d", e)
	}
}
