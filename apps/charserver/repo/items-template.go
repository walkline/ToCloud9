package repo

import "errors"

var (
	ItemsTemplateNotFound = errors.New("item not found")
)

type ItemTemplate struct {
	ID            uint32
	DisplayID     uint32
	InventoryType uint8
}

type ItemsTemplate interface {
	TemplateByID(ID uint32) (*ItemTemplate, error)
}
