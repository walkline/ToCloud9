package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/auctionhouse/repo"
	pbMail "github.com/walkline/ToCloud9/gen/mail/pb"
	"github.com/walkline/ToCloud9/shared/events"
)

const (
	MaxAuctionsPerPage = 50
	MaxGetAllReturn    = 55000
	AuctionSearchDelay = 300
	MinAuctionTime     = 12 * 60 * 60 // 12h in seconds
)

const (
	AuctionActionSell     = 0
	AuctionActionCancel   = 1
	AuctionActionPlaceBid = 2
)

const (
	AuctionMailOutbidded       = 0
	AuctionMailWon             = 1
	AuctionMailSuccessful      = 2
	AuctionMailExpired         = 3
	AuctionMailCancelledBidder = 4
	AuctionMailCanceled        = 5
	AuctionMailSalePending     = 6
)

// Auction house IDs
const (
	AuctionHouseAlliance = 2
	AuctionHouseHorde    = 6
	AuctionHouseNeutral  = 7
)

// Auction house fees
const (
	AuctionCutFaction = 5  // 5% cut for faction auction houses
	AuctionCutNeutral = 15 // 15% cut for neutral auction house
	MinimumOutbid     = 5  // 5% minimum outbid increment
)

// Auction sort columns (from WoW client)
const (
	AuctionSortColumnLevel      = 0
	AuctionSortColumnQuality    = 1
	AuctionSortColumnBuyout     = 2
	AuctionSortColumnTimeLeft   = 3
	AuctionSortColumnBid        = 8
	AuctionSortColumnStackCount = 9
	AuctionSortColumnBuyoutAlt  = 10 // Alternative buyout column
)

type AuctionSortInfo struct {
	Column uint32
	IsDesc bool
}

// CachedAuction is the in-memory representation of an auction entry.
type CachedAuction struct {
	repo.AuctionEntry
}

type AuctionService struct {
	repo           repo.AuctionRepo
	mailClient     pbMail.MailServiceClient
	eventsProducer events.AuctionHouseProducer
	itemTemplates  *repo.ItemTemplateCache

	mu       sync.RWMutex
	auctions map[uint32]map[uint32]*CachedAuction // realmID -> auctionID -> entry

	nextID   uint32
	nextIDMu sync.Mutex
}

func NewAuctionService(r repo.AuctionRepo, mailClient pbMail.MailServiceClient, eventsProducer events.AuctionHouseProducer, itemTemplates *repo.ItemTemplateCache) *AuctionService {
	return &AuctionService{
		repo:           r,
		mailClient:     mailClient,
		eventsProducer: eventsProducer,
		itemTemplates:  itemTemplates,
		auctions:       make(map[uint32]map[uint32]*CachedAuction),
	}
}

func (s *AuctionService) LoadAuctions(ctx context.Context, realmID uint32) error {
	entries, err := s.repo.LoadAllAuctions(ctx, realmID)
	if err != nil {
		return fmt.Errorf("can't load auctions: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.auctions[realmID] == nil {
		s.auctions[realmID] = make(map[uint32]*CachedAuction)
	}

	maxID := uint32(0)
	for i := range entries {
		s.auctions[realmID][entries[i].ID] = &CachedAuction{AuctionEntry: entries[i]}
		if entries[i].ID > maxID {
			maxID = entries[i].ID
		}
	}
	s.nextID = maxID + 1

	log.Info().Uint32("realmID", realmID).Int("count", len(entries)).Msg("Loaded auctions into memory cache")
	return nil
}

func (s *AuctionService) generateAuctionID() uint32 {
	s.nextIDMu.Lock()
	defer s.nextIDMu.Unlock()
	id := s.nextID
	s.nextID++
	return id
}

func (s *AuctionService) SellItem(ctx context.Context, realmID uint32, playerGUID uint64, entry *repo.AuctionEntry) (uint32, error) {
	auctionID := s.generateAuctionID()
	entry.ID = auctionID
	entry.ItemOwner = uint32(playerGUID)

	err := s.repo.InsertAuction(ctx, realmID, entry)
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	if s.auctions[realmID] == nil {
		s.auctions[realmID] = make(map[uint32]*CachedAuction)
	}
	s.auctions[realmID][auctionID] = &CachedAuction{AuctionEntry: *entry}
	s.mu.Unlock()

	// Publish event for other instances to update their cache
	_ = s.eventsProducer.PublishAuctionCreated(&events.AuctionHouseEventAuctionCreatedPayload{
		RealmID:          realmID,
		AuctionID:        entry.ID,
		HouseID:          entry.HouseID,
		ItemGUID:         entry.ItemGUID,
		ItemOwner:        entry.ItemOwner,
		BuyoutPrice:      entry.BuyoutPrice,
		Time:             entry.Time,
		StartBid:         entry.StartBid,
		Deposit:          entry.Deposit,
		ItemEntry:        entry.ItemEntry,
		ItemCount:        entry.ItemCount,
		Charges:          entry.Charges,
		RandomPropertyID: entry.RandomPropertyID,
		SuffixFactor:     entry.SuffixFactor,
		Enchantments:     entry.Enchantments,
		Flags:            entry.Flags,
	})

	return auctionID, nil
}

func (s *AuctionService) PlaceBid(ctx context.Context, realmID uint32, playerGUID uint64, auctionID, price uint32) (isBuyout bool, moneyToDeduct uint32, err error) {
	// Validate and copy auction data under lock
	s.mu.Lock()
	realmAuctions, ok := s.auctions[realmID]
	if !ok {
		s.mu.Unlock()
		return false, 0, ErrAuctionNotFound
	}
	auction, ok := realmAuctions[auctionID]
	if !ok {
		s.mu.Unlock()
		return false, 0, ErrAuctionNotFound
	}

	if auction.ItemOwner == uint32(playerGUID) {
		s.mu.Unlock()
		return false, 0, ErrBidOwnAuction
	}

	if price <= auction.LastBid || price < auction.StartBid {
		s.mu.Unlock()
		return false, 0, ErrBidTooLow
	}

	outBid := calculateOutBid(auction.LastBid)
	if (price < auction.BuyoutPrice || auction.BuyoutPrice == 0) && price < auction.LastBid+outBid {
		s.mu.Unlock()
		return false, 0, ErrBidIncrementTooLow
	}

	oldBidder := auction.BuyGUID
	oldBid := auction.LastBid
	isBuyout = price >= auction.BuyoutPrice && auction.BuyoutPrice > 0

	if !isBuyout {
		// Normal bid - calculate money and update cache
		if auction.BuyGUID != 0 && auction.BuyGUID == uint32(playerGUID) {
			moneyToDeduct = price - auction.LastBid
		} else {
			moneyToDeduct = price
		}

		auction.BuyGUID = uint32(playerGUID)
		auction.LastBid = price

		// Copy data for mail sending outside lock
		auctionCopy := *auction
		s.mu.Unlock()

		// DB update (outside lock)
		err = s.repo.UpdateBid(ctx, realmID, auctionID, uint32(playerGUID), price)
		if err != nil {
			return false, moneyToDeduct, err
		}

		// Return money to old bidder (non-critical, don't fail on error)
		if oldBidder != 0 && oldBidder != uint32(playerGUID) {
			s.sendAuctionOutbidMail(ctx, realmID, &auctionCopy, oldBidder, oldBid)
		}

		// Publish bid event for other instances
		_ = s.eventsProducer.PublishBidPlaced(&events.AuctionHouseEventBidPlacedPayload{
			RealmID:   realmID,
			AuctionID: auctionID,
			BuyGUID:   uint32(playerGUID),
			LastBid:   price,
			IsBuyout:  false,
			OldBidder: oldBidder,
			OldBid:    oldBid,
		})

		return false, moneyToDeduct, nil
	}

	// Buyout path - do all I/O before modifying cache
	if uint32(playerGUID) == auction.BuyGUID {
		moneyToDeduct = auction.BuyoutPrice - auction.LastBid
	} else {
		moneyToDeduct = auction.BuyoutPrice
	}

	// Copy auction data for I/O operations
	auctionCopy := auction.AuctionEntry
	auctionCopy.BuyGUID = uint32(playerGUID)
	auctionCopy.LastBid = auction.BuyoutPrice
	s.mu.Unlock()

	// Send mails first (before DB delete, so we can still see auction in DB if mails fail)
	if err = s.sendAuctionSuccessfulMail(ctx, realmID, &auctionCopy); err != nil {
		log.Error().Err(err).Uint32("auctionID", auctionID).Msg("Failed to send seller payment mail for buyout")
		return true, 0, fmt.Errorf("failed to process auction sale: %w", err)
	}

	if err = s.sendAuctionWonMail(ctx, realmID, &auctionCopy); err != nil {
		log.Error().Err(err).Uint32("auctionID", auctionID).Msg("Failed to send buyer item mail for buyout")
		return true, 0, fmt.Errorf("failed to deliver auction item: %w", err)
	}

	// Return money to old bidder (non-critical)
	if oldBidder != 0 {
		s.sendAuctionOutbidMail(ctx, realmID, &CachedAuction{AuctionEntry: auctionCopy}, oldBidder, oldBid)
	}

	// Delete from DB
	err = s.repo.DeleteAuction(ctx, realmID, auctionID)
	if err != nil {
		log.Error().Err(err).Uint32("auctionID", auctionID).Msg("Failed to delete auction from DB after successful buyout")
		return true, 0, fmt.Errorf("failed to complete auction transaction: %w", err)
	}

	// Now remove from cache (after all I/O succeeded)
	s.mu.Lock()
	if s.auctions[realmID] != nil {
		delete(s.auctions[realmID], auctionID)
	}
	s.mu.Unlock()

	// Publish buyout event for other instances
	_ = s.eventsProducer.PublishBidPlaced(&events.AuctionHouseEventBidPlacedPayload{
		RealmID:   realmID,
		AuctionID: auctionID,
		BuyGUID:   uint32(playerGUID),
		LastBid:   auctionCopy.LastBid,
		IsBuyout:  true,
		OldBidder: oldBidder,
		OldBid:    oldBid,
	})

	return true, moneyToDeduct, nil
}

func (s *AuctionService) CancelAuction(ctx context.Context, realmID uint32, playerGUID uint64, auctionID uint32) (auctionCut uint32, err error) {
	s.mu.Lock()
	realmAuctions, ok := s.auctions[realmID]
	if !ok {
		s.mu.Unlock()
		return 0, ErrAuctionNotFound
	}
	auction, ok := realmAuctions[auctionID]
	if !ok {
		s.mu.Unlock()
		return 0, ErrAuctionNotFound
	}

	if auction.ItemOwner != uint32(playerGUID) {
		s.mu.Unlock()
		return 0, fmt.Errorf("not auction owner")
	}

	auctionCopy := auction.AuctionEntry
	delete(realmAuctions, auctionID)
	s.mu.Unlock()

	if auctionCopy.BuyGUID != 0 {
		auctionCut = calculateAuctionCut(auctionCopy.HouseID, auctionCopy.LastBid)
		s.sendAuctionCancelledToBidderMail(ctx, realmID, &auctionCopy)
	}

	// Send item back to owner
	s.sendAuctionCancelledMail(ctx, realmID, &auctionCopy)

	err = s.repo.DeleteAuction(ctx, realmID, auctionID)

	// Publish cancellation event for other instances
	if err == nil {
		_ = s.eventsProducer.PublishAuctionCanceled(&events.AuctionHouseEventAuctionCanceledPayload{
			RealmID:   realmID,
			AuctionID: auctionID,
		})
	}

	return auctionCut, err
}

func (s *AuctionService) ListItems(realmID uint32, houseID uint32, listFrom uint32, searchedName string,
	levelMin, levelMax, inventoryType, itemClass, itemSubClass, quality uint32,
	getAll bool, sorting []AuctionSortInfo,
) ([]*CachedAuction, uint32) {

	s.mu.RLock()
	defer s.mu.RUnlock()

	realmAuctions := s.auctions[realmID]
	if realmAuctions == nil {
		return nil, 0
	}

	lowerSearch := strings.ToLower(searchedName)

	if getAll {
		var result []*CachedAuction
		for _, a := range realmAuctions {
			if uint32(a.HouseID) != houseID {
				continue
			}
			result = append(result, a)
			if uint32(len(result)) >= MaxGetAllReturn {
				break
			}
		}
		return result, uint32(len(realmAuctions))
	}

	// Collect matching auctions
	var matched []*CachedAuction
	for _, a := range realmAuctions {
		if uint32(a.HouseID) != houseID {
			continue
		}

		// Apply item template filters
		if !s.itemTemplates.MatchesFilters(
			a.ItemEntry,
			lowerSearch,
			levelMin, levelMax,
			inventoryType,
			itemClass, itemSubClass,
			quality,
		) {
			continue
		}

		matched = append(matched, a)
	}

	totalCount := uint32(len(matched))

	if len(sorting) > 0 && len(matched) > MaxAuctionsPerPage {
		sort.Slice(matched, func(i, j int) bool {
			for _, s := range sorting {
				cmp := compareAuctions(matched[i], matched[j], s.Column)
				if cmp == 0 {
					continue
				}
				if s.IsDesc {
					return cmp < 0
				}
				return cmp > 0
			}
			return false
		})
	}

	// Pagination
	if listFrom >= uint32(len(matched)) {
		return nil, totalCount
	}
	matched = matched[listFrom:]
	if uint32(len(matched)) > MaxAuctionsPerPage {
		matched = matched[:MaxAuctionsPerPage]
	}

	return matched, totalCount
}

func (s *AuctionService) ListOwnerItems(realmID uint32, playerGUID uint64, houseID uint32) []*CachedAuction {
	s.mu.RLock()
	defer s.mu.RUnlock()

	realmAuctions := s.auctions[realmID]
	if realmAuctions == nil {
		return nil
	}

	var result []*CachedAuction
	ownerLow := uint32(playerGUID)
	for _, a := range realmAuctions {
		if uint32(a.HouseID) != houseID {
			continue
		}
		if a.ItemOwner == ownerLow {
			result = append(result, a)
		}
	}
	return result
}

func (s *AuctionService) ListBidderItems(realmID uint32, playerGUID uint64, houseID uint32, outbiddedIDs []uint32) []*CachedAuction {
	s.mu.RLock()
	defer s.mu.RUnlock()

	realmAuctions := s.auctions[realmID]
	if realmAuctions == nil {
		return nil
	}

	bidderLow := uint32(playerGUID)
	seen := make(map[uint32]bool)
	var result []*CachedAuction

	// First add outbidded auctions
	for _, id := range outbiddedIDs {
		if a, ok := realmAuctions[id]; ok && uint32(a.HouseID) == houseID {
			result = append(result, a)
			seen[id] = true
		}
	}

	// Then add current bids
	for _, a := range realmAuctions {
		if uint32(a.HouseID) != houseID {
			continue
		}
		if a.BuyGUID == bidderLow && !seen[a.ID] {
			result = append(result, a)
		}
	}
	return result
}

func (s *AuctionService) ProcessExpiredAuctions(ctx context.Context, realmID uint32) {
	now := uint32(time.Now().Unix()) + 60

	// Get candidate expired auctions from cache (eventually consistent)
	s.mu.RLock()
	realmAuctions := s.auctions[realmID]
	var candidates []uint32
	if realmAuctions != nil {
		for _, a := range realmAuctions {
			if a.Time <= now {
				candidates = append(candidates, a.ID)
			}
		}
	}
	s.mu.RUnlock()

	// Process each candidate auction
	// The database delete will act as a distributed lock - only one instance succeeds
	processedCount := 0
	for _, auctionID := range candidates {
		// Try to delete from database - if already deleted by another instance, this fails
		auction, deleted := s.repo.DeleteAuctionIfExists(ctx, realmID, auctionID)
		if !deleted {
			// Another instance already processed this auction
			// Remove from local cache anyway
			s.mu.Lock()
			if s.auctions[realmID] != nil {
				delete(s.auctions[realmID], auctionID)
			}
			s.mu.Unlock()
			continue
		}

		// We successfully claimed this auction - process it
		processedCount++

		// Remove from cache
		s.mu.Lock()
		if s.auctions[realmID] != nil {
			delete(s.auctions[realmID], auctionID)
		}
		s.mu.Unlock()

		// Send mails
		if auction.BuyGUID == 0 {
			// No bidder - return item to owner
			if err := s.sendAuctionExpiredMail(ctx, realmID, auction); err != nil {
				log.Error().Err(err).Uint32("auctionID", auctionID).Msg("Failed to send expired auction mail - item lost")
				// Continue processing - auction is already deleted from DB
			}
		} else {
			// Has bidder - complete the sale
			if err := s.sendAuctionSuccessfulMail(ctx, realmID, auction); err != nil {
				log.Error().Err(err).Uint32("auctionID", auctionID).Msg("Failed to send seller payment for expired auction - money lost")
				// Continue processing - auction is already deleted from DB
			}
			if err := s.sendAuctionWonMail(ctx, realmID, auction); err != nil {
				log.Error().Err(err).Uint32("auctionID", auctionID).Msg("Failed to send buyer item for expired auction - item lost")
				// Continue processing - auction is already deleted from DB
			}
		}

		// Publish expiration event for other instances
		_ = s.eventsProducer.PublishAuctionExpired(&events.AuctionHouseEventAuctionExpiredPayload{
			RealmID:   realmID,
			AuctionID: auctionID,
		})
	}

	if processedCount > 0 {
		log.Info().Int("count", processedCount).Msg("Processed expired auctions")
	}
}

// Mail helper functions

func buildAuctionMailSubject(itemEntry uint32, itemCount uint32, response int) string {
	return fmt.Sprintf("%d:0:%d:0:%d", itemEntry, response, itemCount)
}

func buildAuctionMailBody(guid uint64, bid, buyout, deposit, cut uint32) string {
	return fmt.Sprintf("%016x:%d:%d:%d:%d:0:0", guid, bid, buyout, deposit, cut)
}

func (s *AuctionService) sendAuctionWonMail(ctx context.Context, realmID uint32, a *repo.AuctionEntry) error {
	subject := buildAuctionMailSubject(a.ItemEntry, a.ItemCount, AuctionMailWon)
	body := buildAuctionMailBody(uint64(a.ItemOwner), a.LastBid, a.BuyoutPrice, 0, 0)

	_, err := s.mailClient.Send(ctx, &pbMail.SendRequest{
		RealmID:             realmID,
		SenderGuid:          nil,
		ReceiverGuid:        uint64(a.BuyGUID),
		Subject:             subject,
		Body:                body,
		Stationery:          pbMail.MailStationery_StAuction,
		Type:                pbMail.MailType_Auction,
		DeliveryTimestamp:   time.Now().Unix(),
		ExpirationTimestamp: time.Now().Add(30 * 24 * time.Hour).Unix(),
		Attachments: []*pbMail.ItemAttachment{
			{
				Guid:  uint64(a.ItemGUID),
				Entry: a.ItemEntry,
				Count: a.ItemCount,
			},
		},
	})
	if err != nil {
		log.Error().
			Err(err).
			Uint32("realmID", realmID).
			Uint32("auctionID", a.ID).
			Uint32("buyerGUID", a.BuyGUID).
			Uint32("itemEntry", a.ItemEntry).
			Msg("failed to send auction won mail")
		return fmt.Errorf("failed to send auction won mail: %w", err)
	}
	return nil
}

func (s *AuctionService) sendAuctionSuccessfulMail(ctx context.Context, realmID uint32, a *repo.AuctionEntry) error {
	cut := calculateAuctionCut(a.HouseID, a.LastBid)
	profit := int32(a.LastBid + a.Deposit - cut)
	if profit < 0 {
		profit = 0
	}

	subject := buildAuctionMailSubject(a.ItemEntry, a.ItemCount, AuctionMailSuccessful)
	body := buildAuctionMailBody(uint64(a.BuyGUID), a.LastBid, a.BuyoutPrice, a.Deposit, cut)

	_, err := s.mailClient.Send(ctx, &pbMail.SendRequest{
		RealmID:             realmID,
		SenderGuid:          nil,
		ReceiverGuid:        uint64(a.ItemOwner),
		Subject:             subject,
		Body:                body,
		MoneyToSend:         profit,
		Stationery:          pbMail.MailStationery_StAuction,
		Type:                pbMail.MailType_Auction,
		DeliveryTimestamp:   time.Now().Unix(),
		ExpirationTimestamp: time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	if err != nil {
		log.Error().
			Err(err).
			Uint32("realmID", realmID).
			Uint32("auctionID", a.ID).
			Uint32("sellerGUID", a.ItemOwner).
			Uint32("profit", uint32(profit)).
			Msg("failed to send auction successful mail")
		return fmt.Errorf("failed to send auction successful mail: %w", err)
	}
	return nil
}

func (s *AuctionService) sendAuctionExpiredMail(ctx context.Context, realmID uint32, a *repo.AuctionEntry) error {
	subject := buildAuctionMailSubject(a.ItemEntry, a.ItemCount, AuctionMailExpired)
	body := buildAuctionMailBody(0, 0, a.BuyoutPrice, a.Deposit, 0)

	_, err := s.mailClient.Send(ctx, &pbMail.SendRequest{
		RealmID:             realmID,
		SenderGuid:          nil,
		ReceiverGuid:        uint64(a.ItemOwner),
		Subject:             subject,
		Body:                body,
		Stationery:          pbMail.MailStationery_StAuction,
		Type:                pbMail.MailType_Auction,
		DeliveryTimestamp:   time.Now().Unix(),
		ExpirationTimestamp: time.Now().Add(30 * 24 * time.Hour).Unix(),
		Attachments: []*pbMail.ItemAttachment{
			{
				Guid:  uint64(a.ItemGUID),
				Entry: a.ItemEntry,
				Count: a.ItemCount,
			},
		},
	})
	if err != nil {
		log.Error().
			Err(err).
			Uint32("realmID", realmID).
			Uint32("auctionID", a.ID).
			Uint32("ownerGUID", a.ItemOwner).
			Uint32("itemEntry", a.ItemEntry).
			Msg("failed to send auction expired mail")
		return fmt.Errorf("failed to send auction expired mail: %w", err)
	}
	return nil
}

func (s *AuctionService) sendAuctionOutbidMail(ctx context.Context, realmID uint32, a *CachedAuction, oldBidder uint32, oldBid uint32) {
	subject := buildAuctionMailSubject(a.ItemEntry, a.ItemCount, AuctionMailOutbidded)
	body := buildAuctionMailBody(uint64(a.ItemOwner), a.LastBid, a.BuyoutPrice, a.Deposit, 0)

	_, err := s.mailClient.Send(ctx, &pbMail.SendRequest{
		RealmID:             realmID,
		SenderGuid:          nil,
		ReceiverGuid:        uint64(oldBidder),
		Subject:             subject,
		Body:                body,
		MoneyToSend:         int32(oldBid),
		Stationery:          pbMail.MailStationery_StAuction,
		Type:                pbMail.MailType_Auction,
		DeliveryTimestamp:   time.Now().Unix(),
		ExpirationTimestamp: time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	if err != nil {
		log.Error().
			Err(err).
			Uint32("realmID", realmID).
			Uint32("auctionID", a.ID).
			Uint32("outbidPlayerGUID", oldBidder).
			Uint32("refundAmount", oldBid).
			Msg("failed to send auction outbid mail")
	}
}

func (s *AuctionService) sendAuctionCancelledToBidderMail(ctx context.Context, realmID uint32, a *repo.AuctionEntry) {
	subject := buildAuctionMailSubject(a.ItemEntry, a.ItemCount, AuctionMailCancelledBidder)
	body := buildAuctionMailBody(uint64(a.ItemOwner), a.LastBid, a.BuyoutPrice, a.Deposit, 0)

	_, err := s.mailClient.Send(ctx, &pbMail.SendRequest{
		RealmID:             realmID,
		SenderGuid:          nil,
		ReceiverGuid:        uint64(a.BuyGUID),
		Subject:             subject,
		Body:                body,
		MoneyToSend:         int32(a.LastBid),
		Stationery:          pbMail.MailStationery_StAuction,
		Type:                pbMail.MailType_Auction,
		DeliveryTimestamp:   time.Now().Unix(),
		ExpirationTimestamp: time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	if err != nil {
		log.Error().
			Err(err).
			Uint32("realmID", realmID).
			Uint32("auctionID", a.ID).
			Uint32("bidderGUID", a.BuyGUID).
			Uint32("refundAmount", a.LastBid).
			Msg("failed to send auction cancelled to bidder mail")
	}
}

func (s *AuctionService) sendAuctionCancelledMail(ctx context.Context, realmID uint32, a *repo.AuctionEntry) {
	subject := buildAuctionMailSubject(a.ItemEntry, a.ItemCount, AuctionMailCanceled)
	body := buildAuctionMailBody(0, 0, a.BuyoutPrice, a.Deposit, 0)

	_, err := s.mailClient.Send(ctx, &pbMail.SendRequest{
		RealmID:             realmID,
		SenderGuid:          nil,
		ReceiverGuid:        uint64(a.ItemOwner),
		Subject:             subject,
		Body:                body,
		Stationery:          pbMail.MailStationery_StAuction,
		Type:                pbMail.MailType_Auction,
		DeliveryTimestamp:   time.Now().Unix(),
		ExpirationTimestamp: time.Now().Add(30 * 24 * time.Hour).Unix(),
		Attachments: []*pbMail.ItemAttachment{
			{
				Guid:  uint64(a.ItemGUID),
				Entry: a.ItemEntry,
				Count: a.ItemCount,
			},
		},
	})
	if err != nil {
		log.Error().
			Err(err).
			Uint32("realmID", realmID).
			Uint32("auctionID", a.ID).
			Uint32("ownerGUID", a.ItemOwner).
			Uint32("itemEntry", a.ItemEntry).
			Msg("failed to send auction cancelled mail")
	}
}

// Utility functions

func calculateOutBid(currentBid uint32) uint32 {
	outbid := currentBid * MinimumOutbid / 100
	if outbid == 0 {
		outbid = 1
	}
	return outbid
}

func calculateAuctionCut(houseID uint8, bid uint32) uint32 {
	cutPct := uint32(AuctionCutFaction)
	if houseID == AuctionHouseNeutral {
		cutPct = AuctionCutNeutral
	}
	cut := bid * cutPct / 100
	return cut
}

func compareAuctions(a, b *CachedAuction, column uint32) int {
	switch column {
	case AuctionSortColumnLevel:
		// We don't have item level in cache, so compare by item entry as fallback
		return int(int64(a.ItemEntry) - int64(b.ItemEntry))
	case AuctionSortColumnBuyout, AuctionSortColumnBuyoutAlt:
		if a.BuyoutPrice != b.BuyoutPrice {
			return int(int64(a.BuyoutPrice) - int64(b.BuyoutPrice))
		}
		// If buyout is the same, compare by current bid as tiebreaker
		return int(int64(a.LastBid) - int64(b.LastBid))
	case AuctionSortColumnTimeLeft:
		return int(int64(a.Time) - int64(b.Time))
	case AuctionSortColumnBid:
		bidA := a.LastBid
		if bidA == 0 {
			bidA = a.StartBid
		}
		bidB := b.LastBid
		if bidB == 0 {
			bidB = b.StartBid
		}
		return int(int64(bidA) - int64(bidB))
	case AuctionSortColumnStackCount:
		return int(int64(a.ItemCount) - int64(b.ItemCount))
	}
	return 0
}
