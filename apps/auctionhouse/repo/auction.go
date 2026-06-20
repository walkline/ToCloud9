package repo

import "context"

// AuctionHouseID represents the house faction.
type AuctionHouseID uint8

const (
	AuctionHouseAlliance AuctionHouseID = 2
	AuctionHouseHorde    AuctionHouseID = 6
	AuctionHouseNeutral  AuctionHouseID = 7
)

// AuctionEntry is a single auction in the auctionhouse table.
type AuctionEntry struct {
	ID         uint32
	HouseID    uint8
	ItemGUID   uint32
	ItemOwner  uint32
	BuyoutPrice uint32
	Time       uint32 // unix timestamp
	BuyGUID    uint32
	LastBid    uint32
	StartBid   uint32
	Deposit    uint32

	// Joined from item_instance for search/list purposes
	ItemEntry        uint32
	ItemCount        uint32
	Charges          int32
	RandomPropertyID int32
	SuffixFactor     uint32
	Enchantments     string
	Flags            uint32
}

// AuctionRepo is the interface for auction persistence.
type AuctionRepo interface {
	// LoadAllAuctions loads all auction entries with item info.
	LoadAllAuctions(ctx context.Context, realmID uint32) ([]AuctionEntry, error)

	// InsertAuction inserts a new auction entry.
	InsertAuction(ctx context.Context, realmID uint32, auction *AuctionEntry) error

	// UpdateBid updates bidder and bid price for an auction.
	UpdateBid(ctx context.Context, realmID uint32, auctionID, buyGUID, lastBid uint32) error

	// DeleteAuction deletes an auction by ID.
	DeleteAuction(ctx context.Context, realmID uint32, auctionID uint32) error

	// DeleteAuctionIfExists attempts to delete an auction and returns its data if successful.
	// Returns (auction, true) if deleted, (nil, false) if already deleted by another instance.
	DeleteAuctionIfExists(ctx context.Context, realmID uint32, auctionID uint32) (*AuctionEntry, bool)
}