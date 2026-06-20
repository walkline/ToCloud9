package session

import (
	"context"
	"fmt"
	"time"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbAH "github.com/walkline/ToCloud9/gen/auctionhouse/pb"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

const (
	npcFlagAuctioneer uint32 = 0x00200000

	auctionHouseIDAlliance uint32 = 2
	auctionHouseIDHorde    uint32 = 6
	auctionHouseIDNeutral  uint32 = 7

	maxAuctionEnchantmentSlot = 7
)

func auctionHouseIDForRace(race uint8) uint32 {
	// Alliance races: Human(1), Dwarf(3), NightElf(4), Gnome(7), Draenei(11)
	switch race {
	case 1, 3, 4, 7, 11:
		return auctionHouseIDAlliance
	// Horde races: Orc(2), Undead(5), Tauren(6), Troll(8), BloodElf(10)
	case 2, 5, 6, 8, 10:
		return auctionHouseIDHorde
	default:
		return auctionHouseIDNeutral
	}
}

func (s *GameSession) HandleAuctionHello(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	npcGUID := reader.Uint64()

	canInteract, err := s.CanInteractWithAuctioneer(ctx, npcGUID)
	fmt.Println("canInteract", canInteract)
	if err != nil {
		return err
	}
	if !canInteract {
		return nil
	}

	houseID := auctionHouseIDForRace(s.character.Race)

	wr := packet.NewWriterWithSize(packet.MsgAuctionHello, 12)
	wr.Uint64(npcGUID)
	wr.Uint32(houseID)
	wr.Uint8(1) // AH enabled
	s.gameSocket.Send(wr)
	return nil
}

func (s *GameSession) HandleAuctionSellItem(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	auctioneerGUID := reader.Uint64()
	itemsCount := reader.Uint32()

	if itemsCount > 160 {
		s.sendAuctionCommandResult(0, 0, 2)
		return nil
	}

	type itemSlot struct {
		guid  uint64
		count uint32
	}

	items := make([]itemSlot, itemsCount)
	for i := uint32(0); i < itemsCount; i++ {
		items[i].guid = reader.Uint64()
		items[i].count = reader.Uint32()
		fmt.Printf("DEBUG packet: items[%d].guid=%d, items[%d].count=%d\n", i, items[i].guid, i, items[i].count)
	}

	bid := reader.Uint32()
	buyout := reader.Uint32()
	etime := reader.Uint32()
	fmt.Printf("DEBUG packet: bid=%d, buyout=%d, etime=%d\n", bid, buyout, etime)

	if bid == 0 || etime == 0 {
		return nil
	}

	canInteract, err := s.CanInteractWithAuctioneer(ctx, auctioneerGUID)
	if err != nil || !canInteract {
		return err
	}

	// Validate etime
	etime *= 60
	switch etime {
	case 12 * 60 * 60, 24 * 60 * 60, 48 * 60 * 60:
	default:
		return nil
	}

	houseID := auctionHouseIDForRace(s.character.Race)

	if itemsCount == 0 || items[0].guid == 0 {
		s.sendAuctionCommandResult(0, 0, 1)
		return nil
	}

	// Get item details from game server
	rawGuids := make([]uint64, itemsCount)
	for i := uint32(0); i < itemsCount; i++ {
		rawGuids[i] = items[i].guid
	}

	itemsResp, err := s.gameServerGRPCClient.GetPlayerItemsByGuids(ctx, &pb.GetPlayerItemsByGuidsRequest{
		PlayerGuid: s.character.GUID,
		Guids:      rawGuids,
	})
	if err != nil || len(itemsResp.Items) == 0 {
		s.sendAuctionCommandResult(0, 0, 4)
		return nil
	}

	item := itemsResp.Items[0]
	totalCount := uint32(0)
	for i := uint32(0); i < itemsCount; i++ {
		totalCount += items[i].count
	}
	if totalCount == 0 {
		totalCount = item.Count
	}
	fmt.Printf("DEBUG HandleAuctionSellItem: itemsCount=%d, items[0].count=%d, item.Count=%d, totalCount=%d\n",
		itemsCount, items[0].count, item.Count, totalCount)

	// Remove item from player
	_, err = s.gameServerGRPCClient.RemoveItemsWithGuidsFromPlayer(ctx, &pb.RemoveItemsWithGuidsFromPlayerRequest{
		PlayerGuid:         s.character.GUID,
		Guids:              rawGuids,
		AssignToPlayerGuid: 0,
	})
	if err != nil {
		s.sendAuctionCommandResult(0, 0, 2)
		return fmt.Errorf("can't remove item from player for AH: %w", err)
	}

	// Create auction via AH service
	resp, err := s.auctionHouseServiceClient.SellItem(ctx, &pbAH.AuctionSellItemRequest{
		RealmID:          1, // TODO: use root.RealmID
		PlayerGuid:       s.character.GUID,
		HouseID:          houseID,
		ItemEntry:        item.Entry,
		ItemGuid:         uint64(guid.New(item.Guid).GetCounter()),
		ItemCount:        totalCount,
		Charges:          0,
		RandomPropertyID: item.RandomPropertyID,
		SuffixFactor:     0,
		StartBid:         bid,
		Buyout:           buyout,
		ExpireTimeSecs:   etime,
		Deposit:          0,
		Flags:            item.Flags,
	})
	if err != nil {
		s.sendAuctionCommandResult(0, 0, 2)
		return NewAuctionHouseServiceUnavailableErr(err)
	}

	if resp.Error != pbAH.AuctionHouseError_AH_OK {
		s.sendAuctionCommandResult(0, 0, uint32(resp.Error))
		return nil
	}

	s.sendAuctionCommandResult(resp.AuctionID, 0, 0) // AUCTION_SELL_ITEM, ERR_AUCTION_OK
	return nil
}

func (s *GameSession) HandleAuctionPlaceBid(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	auctioneerGUID := reader.Uint64()
	auctionID := reader.Uint32()
	price := reader.Uint32()

	if auctionID == 0 || price == 0 {
		return nil
	}

	canInteract, err := s.CanInteractWithAuctioneer(ctx, auctioneerGUID)
	if err != nil || !canInteract {
		return err
	}

	houseID := auctionHouseIDForRace(s.character.Race)

	resp, err := s.auctionHouseServiceClient.PlaceBid(ctx, &pbAH.AuctionPlaceBidRequest{
		RealmID:    1,
		PlayerGuid: s.character.GUID,
		HouseID:    houseID,
		AuctionID:  auctionID,
		Price:      price,
	})
	if err != nil {
		s.sendAuctionCommandResult(0, 2, 2)
		return NewAuctionHouseServiceUnavailableErr(err)
	}

	if resp.Error != pbAH.AuctionHouseError_AH_OK {
		s.sendAuctionCommandResult(0, 2, uint32(resp.Error))
		return nil
	}

	// Deduct money from player
	if resp.MoneyToDeduct > 0 {
		_, err = s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pb.ModifyMoneyForPlayerRequest{
			PlayerGuid: s.character.GUID,
			Value:      -int32(resp.MoneyToDeduct),
		})
		if err != nil {
			return fmt.Errorf("can't deduct money for AH bid: %w", err)
		}
	}

	s.sendAuctionCommandResult(resp.AuctionID, 2, 0) // AUCTION_PLACE_BID, ERR_AUCTION_OK
	return nil
}

func (s *GameSession) HandleAuctionRemoveItem(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	auctioneerGUID := reader.Uint64()
	auctionID := reader.Uint32()

	canInteract, err := s.CanInteractWithAuctioneer(ctx, auctioneerGUID)
	if err != nil || !canInteract {
		return err
	}

	houseID := auctionHouseIDForRace(s.character.Race)

	resp, err := s.auctionHouseServiceClient.CancelAuction(ctx, &pbAH.AuctionCancelRequest{
		RealmID:    1,
		PlayerGuid: s.character.GUID,
		HouseID:    houseID,
		AuctionID:  auctionID,
	})
	if err != nil {
		s.sendAuctionCommandResult(0, 1, 2)
		return NewAuctionHouseServiceUnavailableErr(err)
	}

	if resp.Error != pbAH.AuctionHouseError_AH_OK {
		s.sendAuctionCommandResult(0, 1, uint32(resp.Error))
		return nil
	}

	// Deduct auction cut from player
	if resp.AuctionCut > 0 {
		_, err = s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pb.ModifyMoneyForPlayerRequest{
			PlayerGuid: s.character.GUID,
			Value:      -int32(resp.AuctionCut),
		})
		if err != nil {
			return fmt.Errorf("can't deduct auction cut: %w", err)
		}
	}

	s.sendAuctionCommandResult(resp.AuctionID, 1, 0) // AUCTION_CANCEL, ERR_AUCTION_OK
	return nil
}

func (s *GameSession) HandleAuctionListItems(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	_ = reader.Uint64() // auctioneer GUID
	listFrom := reader.Uint32()
	searchedName := reader.String()
	levelMin := reader.Uint8()
	levelMax := reader.Uint8()
	auctionSlotID := reader.Uint32()
	auctionMainCategory := reader.Uint32()
	auctionSubCategory := reader.Uint32()
	quality := reader.Uint32()
	usable := reader.Uint8()
	getAll := reader.Uint8()

	sortOrderCount := reader.Uint8()
	if sortOrderCount > 11 { // AUCTION_SORT_MAX
		return nil
	}

	sorting := make([]*pbAH.AuctionSortOrder, sortOrderCount)
	for i := uint8(0); i < sortOrderCount; i++ {
		sortMode := reader.Uint8()
		isDesc := reader.Uint8()
		sorting[i] = &pbAH.AuctionSortOrder{
			Column: uint32(sortMode),
			IsDesc: isDesc == 1,
		}
	}

	houseID := auctionHouseIDForRace(s.character.Race)

	resp, err := s.auctionHouseServiceClient.ListItems(ctx, &pbAH.AuctionListItemsRequest{
		RealmID:       1,
		PlayerGuid:    s.character.GUID,
		HouseID:       houseID,
		ListFrom:      listFrom,
		SearchedName:  searchedName,
		LevelMin:      uint32(levelMin),
		LevelMax:      uint32(levelMax),
		InventoryType: auctionSlotID,
		ItemClass:     auctionMainCategory,
		ItemSubClass:  auctionSubCategory,
		Quality:       quality,
		GetAll:        getAll != 0,
		Usable:        usable != 0,
		Sorting:       sorting,
	})
	if err != nil {
		return NewAuctionHouseServiceUnavailableErr(err)
	}

	wr := packet.NewWriterWithSize(packet.SMsgAuctionListResult, 0)
	wr.Uint32(uint32(len(resp.Items)))

	now := time.Now().Unix()
	for _, item := range resp.Items {
		writeAuctionItem(wr, item, now)
	}

	wr.Uint32(resp.TotalCount)
	wr.Uint32(resp.SearchDelay)
	s.gameSocket.Send(wr)
	return nil
}

func (s *GameSession) HandleAuctionListOwnerItems(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	_ = reader.Uint64() // auctioneer GUID
	_ = reader.Uint32() // listFrom (not used)

	houseID := auctionHouseIDForRace(s.character.Race)

	resp, err := s.auctionHouseServiceClient.ListOwnerItems(ctx, &pbAH.AuctionListOwnerItemsRequest{
		RealmID:    1,
		PlayerGuid: s.character.GUID,
		HouseID:    houseID,
	})
	if err != nil {
		return NewAuctionHouseServiceUnavailableErr(err)
	}

	wr := packet.NewWriterWithSize(packet.SMsgAuctionOwnerListResult, 0)
	wr.Uint32(uint32(len(resp.Items)))

	now := time.Now().Unix()
	for _, item := range resp.Items {
		writeAuctionItem(wr, item, now)
	}

	wr.Uint32(resp.TotalCount)
	wr.Uint32(resp.SearchDelay)
	s.gameSocket.Send(wr)
	return nil
}

func (s *GameSession) HandleAuctionListBidderItems(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	_ = reader.Uint64() // auctioneer GUID
	_ = reader.Uint32() // listFrom
	outbiddedCount := reader.Uint32()

	if outbiddedCount > 1000 {
		return nil
	}

	outbiddedIDs := make([]uint32, outbiddedCount)
	for i := uint32(0); i < outbiddedCount; i++ {
		outbiddedIDs[i] = reader.Uint32()
	}

	houseID := auctionHouseIDForRace(s.character.Race)

	resp, err := s.auctionHouseServiceClient.ListBidderItems(ctx, &pbAH.AuctionListBidderItemsRequest{
		RealmID:             1,
		PlayerGuid:          s.character.GUID,
		HouseID:             houseID,
		OutbiddedAuctionIDs: outbiddedIDs,
	})
	if err != nil {
		return NewAuctionHouseServiceUnavailableErr(err)
	}

	wr := packet.NewWriterWithSize(packet.SMsgAuctionBidderListResult, 0)
	wr.Uint32(uint32(len(resp.Items)))

	now := time.Now().Unix()
	for _, item := range resp.Items {
		writeAuctionItem(wr, item, now)
	}

	wr.Uint32(resp.TotalCount)
	wr.Uint32(resp.SearchDelay)
	s.gameSocket.Send(wr)
	return nil
}

func (s *GameSession) HandleAuctionListPendingSales(ctx context.Context, p *packet.Packet) error {
	_ = p.Reader().Uint64() // skip

	wr := packet.NewWriterWithSize(packet.SMsgAuctionListPendingSales, 4)
	wr.Uint32(0)
	s.gameSocket.Send(wr)
	return nil
}

func (s *GameSession) sendAuctionCommandResult(auctionID, action, errorCode uint32) {
	wr := packet.NewWriterWithSize(packet.SMsgAuctionCommandResult, 16)
	wr.Uint32(auctionID)
	wr.Uint32(action)
	wr.Uint32(errorCode)
	if errorCode == 0 && action != 0 {
		wr.Uint32(0) // bidError
	}
	s.gameSocket.Send(wr)
}

func writeAuctionItem(wr *packet.Writer, item *pbAH.AuctionItem, now int64) {
	fmt.Printf("DEBUG writeAuctionItem: auctionID=%d, itemEntry=%d, itemCount=%d, charges=%d, buyout=%d, startBid=%d, bid=%d, ownerGuid=%d\n",
		item.AuctionID, item.ItemEntry, item.ItemCount, item.Charges, item.Buyout, item.StartBid, item.Bid, item.OwnerGuid)
	wr.Uint32(item.AuctionID)
	wr.Uint32(item.ItemEntry)

	// Write enchantments (7 slots for auction house packets in 3.3.5)
	enchCount := len(item.Enchantments)
	for i := 0; i < maxAuctionEnchantmentSlot; i++ {
		if i < enchCount {
			wr.Uint32(item.Enchantments[i].Id)
			wr.Uint32(item.Enchantments[i].Duration)
			wr.Uint32(item.Enchantments[i].Charges)
		} else {
			wr.Uint32(0)
			wr.Uint32(0)
			wr.Uint32(0)
		}
	}

	wr.Int32(item.RandomPropertyID)
	wr.Uint32(item.SuffixFactor)
	wr.Uint32(item.ItemCount)
	wr.Int32(item.Charges)
	wr.Uint32(0) // item flags (client ignores)
	wr.Uint64(item.OwnerGuid)

	// Current bid (or start bid if no bids yet)
	currentBid := item.Bid
	if currentBid == 0 {
		currentBid = item.StartBid
	}
	wr.Uint32(currentBid)

	// Minimum outbid increment
	outbid := uint32(0)
	if item.Bid > 0 {
		outbid = item.Bid * 5 / 100
		if outbid == 0 {
			outbid = 1
		}
	}
	wr.Uint32(outbid)
	wr.Uint32(item.Buyout)

	// Time left in milliseconds
	timeLeft := item.ExpireTime - now
	if timeLeft < 0 {
		timeLeft = 0
	}
	wr.Uint32(uint32(timeLeft * 1000))
	wr.Uint64(item.BidderGuid)
	wr.Uint32(item.Bid)
}

func (s *GameSession) CanInteractWithAuctioneer(ctx context.Context, npcGUID uint64) (bool, error) {
	resp, err := s.gameServerGRPCClient.CanPlayerInteractWithNPC(ctx, &pb.CanPlayerInteractWithNPCRequest{
		PlayerGuid: s.character.GUID,
		NpcGuid:    npcGUID,
		NpcFlags:   npcFlagAuctioneer,
	})
	if err != nil {
		return false, fmt.Errorf("failed to make CanPlayerInteractWithNPC request for auctioneer: %w", err)
	}
	return resp.CanInteract, nil
}

func NewAuctionHouseServiceUnavailableErr(err error) error {
	return &UserFriendlyError{
		UserError: "Auction House service unavailable. Try again later.",
		RealError: err,
	}
}
