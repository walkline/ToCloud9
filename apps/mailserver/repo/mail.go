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
	MailTypeAuction        MailType = 1 + iota
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

type MailStationeryType uint8

const (
	MailStationeryTypeTest      MailStationeryType = 1
	MailStationeryTypeDefault   MailStationeryType = 41
	MailStationeryTypeGM        MailStationeryType = 61
	MailStationeryTypeAuction   MailStationeryType = 62
	MailStationeryTypeValentine MailStationeryType = 64
	MailStationeryTypeChristmas MailStationeryType = 65
	MailStationeryTypeOrphan    MailStationeryType = 67
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
	HasItemAttachments  bool

	Attachments []ItemAttachment
}

// MailRepo interface to interact with mail storage.
//
//go:generate mockery --name=MailRepo
type MailRepo interface {
	// AddMail creates mail and mail items, sets mail ID into `mail` object.
	AddMail(ctx context.Context, realmID uint32, mail *Mail) error

	// MailByID returns mail with attachments,
	MailByID(ctx context.Context, realmID uint32, mailID uint) (*Mail, error)

	// MailListForPlayer returns list of mails with attachments for given player.
	MailListForPlayer(ctx context.Context, realmID uint32, playerGUID uint64) ([]Mail, error)

	// UpdateMailFlagsMaskForPlayer updates mail flags for given mailID and receiverID.
	UpdateMailFlagsMaskForPlayer(ctx context.Context, realmID uint32, playerGUID uint64, mailID uint, mask MailFlagMask) error

	// UpdateMailWithoutAttachments updates mail object, ignores mail attachments.
	UpdateMailWithoutAttachments(ctx context.Context, realmID uint32, mail *Mail) error

	// UpdateMailItemsOwner updates owner for mail items by mailID.
	UpdateMailItemsOwner(ctx context.Context, realmID uint32, mailID uint, newReceiverID uint64) error

	// RemoveMailItem removes mail item with given id.
	RemoveMailItem(ctx context.Context, realmID uint32, mailID uint, mailItemGUID uint64) error

	// RemoveMailItemForPlayer removes mail item with given id and receiver id.
	RemoveMailItemForPlayer(ctx context.Context, realmID uint32, mailID uint, mailItemGUID, playerGUID uint64) error

	// RemoveMailMoney removes mail money with given id.
	RemoveMailMoney(ctx context.Context, realmID uint32, mailID uint) (int32, error)

	// RemoveMailMoneyForPlayer removes mail money with given id and receiver id.
	RemoveMailMoneyForPlayer(ctx context.Context, realmID uint32, mailID uint, playerGUID uint64) (int32, error)

	// ExpiredMails returns list of expired mails.
	ExpiredMails(ctx context.Context, realmID uint32) ([]Mail, error)

	// MailItemsIDsByMailIDs returns items ids for given mail ids.
	MailItemsIDsByMailIDs(ctx context.Context, realmID uint32, IDs []uint) ([]uint64, error)

	// DeleteMailsWithoutAttachments deletes mails with given ids, ignores mails attachments.
	DeleteMailsWithoutAttachments(ctx context.Context, realmID uint32, IDs []uint) error

	// DeleteMailItemsWithIDs deletes mail items with given ids.
	DeleteMailItemsWithIDs(ctx context.Context, realmID uint32, itemIDs []uint64) error

	// DeleteItemsWithIDs deletes items with given ids.
	DeleteItemsWithIDs(ctx context.Context, realmID uint32, itemIDs []uint64) error
}
