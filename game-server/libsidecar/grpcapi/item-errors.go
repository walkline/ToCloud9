package grpcapi

import "fmt"

type ItemError uint8

const (
	ItemErrorNoHandler ItemError = iota + 1
	ItemErrorNoPlayer
	ItemErrorNoInventorySpace
	ItemErrorUnknownTemplate
	ItemErrorFailedToCreateItem
)

func (e ItemError) Error() string {
	switch e {
	case ItemErrorNoHandler:
		return "no handler defined"
	case ItemErrorNoPlayer:
		return "player with given guid not found"
	case ItemErrorNoInventorySpace:
		return "no inventory space"
	case ItemErrorUnknownTemplate:
		return "unknown item template"
	case ItemErrorFailedToCreateItem:
		return "failed to create item"
	default:
		return fmt.Sprintf("unk item error %d", e)
	}
}
