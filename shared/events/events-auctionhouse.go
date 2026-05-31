package events

const (
	AuctionHouseEventAuctionCreated  = "auctionhouse.auction.created"
	AuctionHouseEventBidPlaced       = "auctionhouse.bid.placed"
	AuctionHouseEventAuctionCanceled = "auctionhouse.auction.canceled"
	AuctionHouseEventAuctionExpired  = "auctionhouse.auction.expired"
)

type AuctionHouseEventAuctionCreatedPayload struct {
	RealmID          uint32 `json:"realm_id"`
	AuctionID        uint32 `json:"auction_id"`
	HouseID          uint8  `json:"house_id"`
	ItemGUID         uint32 `json:"item_guid"`
	ItemOwner        uint32 `json:"item_owner"`
	BuyoutPrice      uint32 `json:"buyout_price"`
	Time             uint32 `json:"time"`
	StartBid         uint32 `json:"start_bid"`
	Deposit          uint32 `json:"deposit"`
	ItemEntry        uint32 `json:"item_entry"`
	ItemCount        uint32 `json:"item_count"`
	Charges          int32  `json:"charges"`
	RandomPropertyID int32  `json:"random_property_id"`
	SuffixFactor     uint32 `json:"suffix_factor"`
	Enchantments     string `json:"enchantments"`
	Flags            uint32 `json:"flags"`
}

type AuctionHouseEventBidPlacedPayload struct {
	RealmID    uint32 `json:"realm_id"`
	AuctionID  uint32 `json:"auction_id"`
	BuyGUID    uint32 `json:"buy_guid"`
	LastBid    uint32 `json:"last_bid"`
	IsBuyout   bool   `json:"is_buyout"`
	OldBidder  uint32 `json:"old_bidder,omitempty"`
	OldBid     uint32 `json:"old_bid,omitempty"`
}

type AuctionHouseEventAuctionCanceledPayload struct {
	RealmID   uint32 `json:"realm_id"`
	AuctionID uint32 `json:"auction_id"`
}

type AuctionHouseEventAuctionExpiredPayload struct {
	RealmID   uint32 `json:"realm_id"`
	AuctionID uint32 `json:"auction_id"`
}
