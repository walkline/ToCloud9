package grpcapi

import "fmt"

type InteractionsError uint8

const (
	InteractionsErrorNoHandler InteractionsError = iota + 1
	InteractionsErrorNoPlayer
)

func (e InteractionsError) Error() string {
	switch e {
	case InteractionsErrorNoHandler:
		return "no handler defined"
	case InteractionsErrorNoPlayer:
		return "player with given guid not found"
	default:
		return fmt.Sprintf("unk interaction error %d", e)
	}
}
