package grpcapi

import "fmt"

type MoneyError uint8

const (
	MoneyErrorNoHandler MoneyError = iota + 1
	MoneyErrorNoPlayer
	MoneyErrorTooMuchMoney
)

func (e MoneyError) Error() string {
	switch e {
	case MoneyErrorNoHandler:
		return "no handler defined"
	case MoneyErrorNoPlayer:
		return "player with given guid not found"
	case MoneyErrorTooMuchMoney:
		return "too much money"
	default:
		return fmt.Sprintf("unk item error %d", e)
	}
}
