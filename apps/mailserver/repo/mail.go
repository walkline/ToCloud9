package repo

import "context"

type ItemAttachment struct {
	GUID             uint64
	OwnerGUID        uint64
	Entry            uint
	Pos              uint8
	Count            int
	Charges          int
	Flags            uint
	RandomPropertyID uint
	PropertySeed     uint
	Durability       int
	Text             string
}

type MailType uint8

const (
	MailTypePlayerToPlayer MailType = iota
	MailTypeAuction
	MailTypeCreature
	MailTypeGameObject
	MailTypeCalendar
)

type MailFlagMask uint16

const (
	MailFlagNone MailFlagMask = 0
	MailFlagRead MailFlagMask = 1 << (iota - 1)
	MailFlagReturned
	MailFlagCopied
	MailFlagCashOnDelivery
	MailFlagHasBody
)

type Mail struct {
	ID           uint
	Type         MailType
	Stationery   uint8
	TemplateID   uint32
	SenderGuid   uint64
	ReceiverGuid uint64

	FlagsMask uint16

	Subject string
	Body    string

	MoneyToSend    int32
	CashOnDelivery int32

	DeliveryTimestamp   int64
	ExpirationTimestamp int64

	Attachments []ItemAttachment
}

type MailRepo interface {
	AddMail(ctx context.Context, realmID uint32, mail *Mail) error
	MailByID(ctx context.Context, realmID uint32, mailID uint) (*Mail, error)
	MailListForPlayer(ctx context.Context, realmID uint32, playerGUID uint64) ([]Mail, error)
	UpdateMailFlagsMaskForPlayer(ctx context.Context, realmID uint32, playerGUID uint64, mailID uint, mask MailFlagMask) error
	RemoveMailItem(ctx context.Context, realmID uint32, mailID uint, mailItemGUID uint64) error
	RemoveMailItemForPlayer(ctx context.Context, realmID uint32, mailID uint, mailItemGUID, playerGUID uint64) error
	RemoveMailMoney(ctx context.Context, realmID uint32, mailID uint) (int32, error)
	RemoveMailMoneyForPlayer(ctx context.Context, realmID uint32, mailID uint, playerGUID uint64) (int32, error)
}
