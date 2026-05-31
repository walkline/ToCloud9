package service

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/apps/auctionhouse/repo"
	pbMail "github.com/walkline/ToCloud9/gen/mail/pb"
	"github.com/walkline/ToCloud9/shared/events"
)

// Mock implementations
type mockAuctionRepo struct {
	auctions map[uint32]repo.AuctionEntry
}

func (m *mockAuctionRepo) LoadAllAuctions(ctx context.Context, realmID uint32) ([]repo.AuctionEntry, error) {
	var result []repo.AuctionEntry
	for _, a := range m.auctions {
		result = append(result, a)
	}
	return result, nil
}

func (m *mockAuctionRepo) InsertAuction(ctx context.Context, realmID uint32, entry *repo.AuctionEntry) error {
	m.auctions[entry.ID] = *entry
	return nil
}

func (m *mockAuctionRepo) UpdateBid(ctx context.Context, realmID uint32, auctionID uint32, bidderGUID uint32, bid uint32) error {
	if a, ok := m.auctions[auctionID]; ok {
		a.BuyGUID = bidderGUID
		a.LastBid = bid
		m.auctions[auctionID] = a
	}
	return nil
}

func (m *mockAuctionRepo) DeleteAuction(ctx context.Context, realmID uint32, auctionID uint32) error {
	delete(m.auctions, auctionID)
	return nil
}

func (m *mockAuctionRepo) DeleteAuctionIfExists(ctx context.Context, realmID uint32, auctionID uint32) (*repo.AuctionEntry, bool) {
	if a, ok := m.auctions[auctionID]; ok {
		delete(m.auctions, auctionID)
		return &a, true
	}
	return nil, false
}

type mockMailClient struct{}

func (m *mockMailClient) Send(ctx context.Context, req *pbMail.SendRequest, opts ...grpc.CallOption) (*pbMail.SendResponse, error) {
	return &pbMail.SendResponse{}, nil
}

func (m *mockMailClient) DeleteMail(ctx context.Context, req *pbMail.DeleteMailRequest, opts ...grpc.CallOption) (*pbMail.DeleteMailResponse, error) {
	return &pbMail.DeleteMailResponse{}, nil
}

func (m *mockMailClient) MailByID(ctx context.Context, req *pbMail.MailByIDRequest, opts ...grpc.CallOption) (*pbMail.MailByIDResponse, error) {
	return &pbMail.MailByIDResponse{}, nil
}

func (m *mockMailClient) MailsForPlayer(ctx context.Context, req *pbMail.MailsForPlayerRequest, opts ...grpc.CallOption) (*pbMail.MailsForPlayerResponse, error) {
	return &pbMail.MailsForPlayerResponse{}, nil
}

func (m *mockMailClient) MarkAsReadForPlayer(ctx context.Context, req *pbMail.MarkAsReadForPlayerRequest, opts ...grpc.CallOption) (*pbMail.MarkAsReadForPlayerResponse, error) {
	return &pbMail.MarkAsReadForPlayerResponse{}, nil
}

func (m *mockMailClient) RemoveMailItem(ctx context.Context, req *pbMail.RemoveMailItemRequest, opts ...grpc.CallOption) (*pbMail.RemoveMailItemResponse, error) {
	return &pbMail.RemoveMailItemResponse{}, nil
}

func (m *mockMailClient) RemoveMailMoney(ctx context.Context, req *pbMail.RemoveMailMoneyRequest, opts ...grpc.CallOption) (*pbMail.RemoveMailMoneyResponse, error) {
	return &pbMail.RemoveMailMoneyResponse{}, nil
}

type mockEventsProducer struct{}

func (m *mockEventsProducer) PublishAuctionCreated(payload *events.AuctionHouseEventAuctionCreatedPayload) error {
	return nil
}

func (m *mockEventsProducer) PublishBidPlaced(payload *events.AuctionHouseEventBidPlacedPayload) error {
	return nil
}

func (m *mockEventsProducer) PublishAuctionCanceled(payload *events.AuctionHouseEventAuctionCanceledPayload) error {
	return nil
}

func (m *mockEventsProducer) PublishAuctionExpired(payload *events.AuctionHouseEventAuctionExpiredPayload) error {
	return nil
}

func newTestService() *AuctionService {
	mockRepo := &mockAuctionRepo{auctions: make(map[uint32]repo.AuctionEntry)}
	mockMail := &mockMailClient{}
	mockEvents := &mockEventsProducer{}
	mockTemplates := &repo.ItemTemplateCache{}

	return NewAuctionService(mockRepo, mockMail, mockEvents, mockTemplates)
}

// Test PlaceBid - Normal Bid
func TestPlaceBid_NormalBid(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	realmID := uint32(1)

	// Create auction
	auction := &repo.AuctionEntry{
		HouseID:     AuctionHouseAlliance,
		ItemGUID:    100,
		ItemOwner:   1000,
		StartBid:    100,
		BuyoutPrice: 500,
		Time:        uint32(time.Now().Unix()) + 3600,
		ItemEntry:   12345,
		ItemCount:   1,
	}

	auctionID, err := svc.SellItem(ctx, realmID, uint64(auction.ItemOwner), auction)
	if err != nil {
		t.Fatalf("Failed to create auction: %v", err)
	}

	// Place bid
	bidderGUID := uint64(2000)
	bidPrice := uint32(150)

	isBuyout, moneyToDeduct, err := svc.PlaceBid(ctx, realmID, bidderGUID, auctionID, bidPrice)
	if err != nil {
		t.Fatalf("Failed to place bid: %v", err)
	}

	if isBuyout {
		t.Error("Expected normal bid, got buyout")
	}

	if moneyToDeduct != bidPrice {
		t.Errorf("Expected money to deduct %d, got %d", bidPrice, moneyToDeduct)
	}

	// Verify auction updated
	svc.mu.RLock()
	a := svc.auctions[realmID][auctionID]
	svc.mu.RUnlock()

	if a.BuyGUID != uint32(bidderGUID) {
		t.Errorf("Expected bidder GUID %d, got %d", bidderGUID, a.BuyGUID)
	}

	if a.LastBid != bidPrice {
		t.Errorf("Expected last bid %d, got %d", bidPrice, a.LastBid)
	}
}

// Test PlaceBid - Buyout
func TestPlaceBid_Buyout(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	realmID := uint32(1)

	// Create auction
	auction := &repo.AuctionEntry{
		HouseID:     AuctionHouseAlliance,
		ItemGUID:    100,
		ItemOwner:   1000,
		StartBid:    100,
		BuyoutPrice: 500,
		Time:        uint32(time.Now().Unix()) + 3600,
		ItemEntry:   12345,
		ItemCount:   1,
	}

	auctionID, err := svc.SellItem(ctx, realmID, uint64(auction.ItemOwner), auction)
	if err != nil {
		t.Fatalf("Failed to create auction: %v", err)
	}

	// Place buyout bid
	bidderGUID := uint64(2000)
	bidPrice := auction.BuyoutPrice

	isBuyout, moneyToDeduct, err := svc.PlaceBid(ctx, realmID, bidderGUID, auctionID, bidPrice)
	if err != nil {
		t.Fatalf("Failed to place buyout: %v", err)
	}

	if !isBuyout {
		t.Error("Expected buyout, got normal bid")
	}

	if moneyToDeduct != bidPrice {
		t.Errorf("Expected money to deduct %d, got %d", bidPrice, moneyToDeduct)
	}

	// Verify auction removed from cache
	svc.mu.RLock()
	_, exists := svc.auctions[realmID][auctionID]
	svc.mu.RUnlock()

	if exists {
		t.Error("Expected auction to be removed after buyout")
	}
}

// Test PlaceBid - Bid on own auction
func TestPlaceBid_OwnAuction(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	realmID := uint32(1)

	// Create auction
	auction := &repo.AuctionEntry{
		HouseID:     AuctionHouseAlliance,
		ItemGUID:    100,
		ItemOwner:   1000,
		StartBid:    100,
		BuyoutPrice: 500,
		Time:        uint32(time.Now().Unix()) + 3600,
		ItemEntry:   12345,
		ItemCount:   1,
	}

	auctionID, err := svc.SellItem(ctx, realmID, uint64(auction.ItemOwner), auction)
	if err != nil {
		t.Fatalf("Failed to create auction: %v", err)
	}

	// Try to bid on own auction
	_, _, err = svc.PlaceBid(ctx, realmID, uint64(auction.ItemOwner), auctionID, 150)
	if err != ErrBidOwnAuction {
		t.Errorf("Expected ErrBidOwnAuction, got %v", err)
	}
}

// Test PlaceBid - Bid too low
func TestPlaceBid_BidTooLow(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	realmID := uint32(1)

	// Create auction with existing bid
	auction := &repo.AuctionEntry{
		HouseID:     AuctionHouseAlliance,
		ItemGUID:    100,
		ItemOwner:   1000,
		StartBid:    100,
		BuyoutPrice: 500,
		Time:        uint32(time.Now().Unix()) + 3600,
		ItemEntry:   12345,
		ItemCount:   1,
	}

	auctionID, err := svc.SellItem(ctx, realmID, uint64(auction.ItemOwner), auction)
	if err != nil {
		t.Fatalf("Failed to create auction: %v", err)
	}

	// Place first bid
	_, _, err = svc.PlaceBid(ctx, realmID, 2000, auctionID, 150)
	if err != nil {
		t.Fatalf("Failed to place first bid: %v", err)
	}

	// Try to bid lower than current bid
	_, _, err = svc.PlaceBid(ctx, realmID, 3000, auctionID, 140)
	if err != ErrBidTooLow {
		t.Errorf("Expected ErrBidTooLow, got %v", err)
	}
}

// Test PlaceBid - Increment too small
func TestPlaceBid_IncrementTooSmall(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	realmID := uint32(1)

	// Create auction
	auction := &repo.AuctionEntry{
		HouseID:     AuctionHouseAlliance,
		ItemGUID:    100,
		ItemOwner:   1000,
		StartBid:    100,
		BuyoutPrice: 500,
		Time:        uint32(time.Now().Unix()) + 3600,
		ItemEntry:   12345,
		ItemCount:   1,
	}

	auctionID, err := svc.SellItem(ctx, realmID, uint64(auction.ItemOwner), auction)
	if err != nil {
		t.Fatalf("Failed to create auction: %v", err)
	}

	// Place first bid
	_, _, err = svc.PlaceBid(ctx, realmID, 2000, auctionID, 100)
	if err != nil {
		t.Fatalf("Failed to place first bid: %v", err)
	}

	// Try to bid with insufficient increment (need 5% = 5 copper minimum)
	_, _, err = svc.PlaceBid(ctx, realmID, 3000, auctionID, 101)
	if err != ErrBidIncrementTooLow {
		t.Errorf("Expected ErrBidIncrementTooLow, got %v", err)
	}

	// Valid increment should work
	validBid := uint32(100 + calculateOutBid(100))
	_, _, err = svc.PlaceBid(ctx, realmID, 3000, auctionID, validBid)
	if err != nil {
		t.Errorf("Valid increment failed: %v", err)
	}
}

// Test CancelAuction
func TestCancelAuction(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	realmID := uint32(1)

	// Create auction
	auction := &repo.AuctionEntry{
		HouseID:     AuctionHouseAlliance,
		ItemGUID:    100,
		ItemOwner:   1000,
		StartBid:    100,
		BuyoutPrice: 500,
		Deposit:     10,
		Time:        uint32(time.Now().Unix()) + 3600,
		ItemEntry:   12345,
		ItemCount:   1,
	}

	auctionID, err := svc.SellItem(ctx, realmID, uint64(auction.ItemOwner), auction)
	if err != nil {
		t.Fatalf("Failed to create auction: %v", err)
	}

	// Cancel auction
	auctionCut, err := svc.CancelAuction(ctx, realmID, uint64(auction.ItemOwner), auctionID)
	if err != nil {
		t.Fatalf("Failed to cancel auction: %v", err)
	}

	// No bidder, so no cut
	if auctionCut != 0 {
		t.Errorf("Expected no auction cut without bidder, got %d", auctionCut)
	}

	// Verify auction removed
	svc.mu.RLock()
	_, exists := svc.auctions[realmID][auctionID]
	svc.mu.RUnlock()

	if exists {
		t.Error("Expected auction to be removed after cancellation")
	}
}

// Test CancelAuction - with existing bid
func TestCancelAuction_WithBid(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	realmID := uint32(1)

	// Create auction
	auction := &repo.AuctionEntry{
		HouseID:     AuctionHouseAlliance,
		ItemGUID:    100,
		ItemOwner:   1000,
		StartBid:    100,
		BuyoutPrice: 500,
		Deposit:     10,
		Time:        uint32(time.Now().Unix()) + 3600,
		ItemEntry:   12345,
		ItemCount:   1,
	}

	auctionID, err := svc.SellItem(ctx, realmID, uint64(auction.ItemOwner), auction)
	if err != nil {
		t.Fatalf("Failed to create auction: %v", err)
	}

	// Place bid
	_, _, err = svc.PlaceBid(ctx, realmID, 2000, auctionID, 150)
	if err != nil {
		t.Fatalf("Failed to place bid: %v", err)
	}

	// Cancel auction
	auctionCut, err := svc.CancelAuction(ctx, realmID, uint64(auction.ItemOwner), auctionID)
	if err != nil {
		t.Fatalf("Failed to cancel auction: %v", err)
	}

	// Should have auction cut
	expectedCut := calculateAuctionCut(auction.HouseID, 150)
	if auctionCut != expectedCut {
		t.Errorf("Expected auction cut %d, got %d", expectedCut, auctionCut)
	}
}

// Test realm isolation
func TestRealmIsolation(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	// Create auctions on different realms
	auction1 := &repo.AuctionEntry{
		HouseID:     AuctionHouseAlliance,
		ItemGUID:    100,
		ItemOwner:   1000,
		StartBid:    100,
		BuyoutPrice: 500,
		Time:        uint32(time.Now().Unix()) + 3600,
		ItemEntry:   12345,
		ItemCount:   1,
	}

	auction2 := &repo.AuctionEntry{
		HouseID:     AuctionHouseHorde,
		ItemGUID:    200,
		ItemOwner:   2000,
		StartBid:    200,
		BuyoutPrice: 600,
		Time:        uint32(time.Now().Unix()) + 3600,
		ItemEntry:   54321,
		ItemCount:   1,
	}

	auctionID1, _ := svc.SellItem(ctx, 1, uint64(auction1.ItemOwner), auction1)
	auctionID2, _ := svc.SellItem(ctx, 2, uint64(auction2.ItemOwner), auction2)

	// Try to bid on realm 2 auction from realm 1 context - should fail
	_, _, err := svc.PlaceBid(ctx, 1, 3000, auctionID2, 300)
	if err != ErrAuctionNotFound {
		t.Errorf("Expected ErrAuctionNotFound when accessing cross-realm auction, got %v", err)
	}

	// Bid on correct realm should work
	_, _, err = svc.PlaceBid(ctx, 1, 3000, auctionID1, 150)
	if err != nil {
		t.Errorf("Failed to bid on correct realm: %v", err)
	}

	// List items should only show realm-specific auctions
	items, _ := svc.ListItems(1, AuctionHouseAlliance, 0, "", 0, 0, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, true, nil)
	if len(items) != 1 {
		t.Errorf("Expected 1 auction on realm 1, got %d", len(items))
	}

	items, _ = svc.ListItems(2, AuctionHouseHorde, 0, "", 0, 0, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, true, nil)
	if len(items) != 1 {
		t.Errorf("Expected 1 auction on realm 2, got %d", len(items))
	}
}

// Test auction house fee calculation
func TestAuctionCutCalculation(t *testing.T) {
	tests := []struct {
		name     string
		houseID  uint8
		bid      uint32
		expected uint32
	}{
		{"Alliance AH 5%", AuctionHouseAlliance, 1000, 50},
		{"Horde AH 5%", AuctionHouseHorde, 1000, 50},
		{"Neutral AH 15%", AuctionHouseNeutral, 1000, 150},
		{"Small bid faction", AuctionHouseAlliance, 10, 0},
		{"Large bid neutral", AuctionHouseNeutral, 10000, 1500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cut := calculateAuctionCut(tt.houseID, tt.bid)
			if cut != tt.expected {
				t.Errorf("Expected cut %d, got %d", tt.expected, cut)
			}
		})
	}
}

// Test outbid calculation
func TestOutbidCalculation(t *testing.T) {
	tests := []struct {
		currentBid uint32
		expected   uint32
	}{
		{100, 5},   // 5% of 100
		{1000, 50}, // 5% of 1000
		{10, 1},    // minimum 1 copper
		{5, 1},     // minimum 1 copper
		{0, 1},     // minimum 1 copper
	}

	for _, tt := range tests {
		outbid := calculateOutBid(tt.currentBid)
		if outbid != tt.expected {
			t.Errorf("For bid %d, expected outbid %d, got %d", tt.currentBid, tt.expected, outbid)
		}
	}
}
