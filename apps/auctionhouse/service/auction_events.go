package service

import (
	"github.com/rs/zerolog/log"
	"github.com/walkline/ToCloud9/apps/auctionhouse/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

// HandleAuctionCreated processes auction creation events from other instances
func (s *AuctionService) HandleAuctionCreated(payload *events.AuctionHouseEventAuctionCreatedPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.auctions[payload.RealmID] == nil {
		s.auctions[payload.RealmID] = make(map[uint32]*CachedAuction)
	}

	// Don't add if already exists (could be from this instance)
	if _, exists := s.auctions[payload.RealmID][payload.AuctionID]; exists {
		return
	}

	s.auctions[payload.RealmID][payload.AuctionID] = &CachedAuction{
		AuctionEntry: repo.AuctionEntry{
			ID:               payload.AuctionID,
			HouseID:          payload.HouseID,
			ItemGUID:         payload.ItemGUID,
			ItemOwner:        payload.ItemOwner,
			BuyoutPrice:      payload.BuyoutPrice,
			Time:             payload.Time,
			BuyGUID:          0,
			LastBid:          0,
			StartBid:         payload.StartBid,
			Deposit:          payload.Deposit,
			ItemEntry:        payload.ItemEntry,
			ItemCount:        payload.ItemCount,
			Charges:          payload.Charges,
			RandomPropertyID: payload.RandomPropertyID,
			SuffixFactor:     payload.SuffixFactor,
			Enchantments:     payload.Enchantments,
			Flags:            payload.Flags,
		},
	}

	log.Debug().Uint32("realmID", payload.RealmID).Uint32("auctionID", payload.AuctionID).Msg("Auction added to cache via event")
}

// HandleBidPlaced processes bid events from other instances
func (s *AuctionService) HandleBidPlaced(payload *events.AuctionHouseEventBidPlacedPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()

	realmAuctions := s.auctions[payload.RealmID]
	if realmAuctions == nil {
		log.Warn().Uint32("realmID", payload.RealmID).Uint32("auctionID", payload.AuctionID).Msg("Bid event for unknown realm")
		return
	}

	auction, ok := realmAuctions[payload.AuctionID]
	if !ok {
		log.Warn().Uint32("realmID", payload.RealmID).Uint32("auctionID", payload.AuctionID).Msg("Bid event for unknown auction")
		return
	}

	auction.BuyGUID = payload.BuyGUID
	auction.LastBid = payload.LastBid

	// If buyout, remove from cache (will be deleted from DB)
	if payload.IsBuyout {
		delete(realmAuctions, payload.AuctionID)
		log.Debug().Uint32("realmID", payload.RealmID).Uint32("auctionID", payload.AuctionID).Msg("Auction bought out, removed from cache via event")
	} else {
		log.Debug().Uint32("realmID", payload.RealmID).Uint32("auctionID", payload.AuctionID).Uint32("bid", payload.LastBid).Msg("Bid updated via event")
	}
}

// HandleAuctionCanceled processes auction cancellation events
func (s *AuctionService) HandleAuctionCanceled(payload *events.AuctionHouseEventAuctionCanceledPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.auctions[payload.RealmID] != nil {
		delete(s.auctions[payload.RealmID], payload.AuctionID)
	}
	log.Debug().Uint32("realmID", payload.RealmID).Uint32("auctionID", payload.AuctionID).Msg("Auction canceled, removed from cache via event")
}

// HandleAuctionExpired processes auction expiration events
func (s *AuctionService) HandleAuctionExpired(payload *events.AuctionHouseEventAuctionExpiredPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.auctions[payload.RealmID] != nil {
		delete(s.auctions[payload.RealmID], payload.AuctionID)
	}
	log.Debug().Uint32("realmID", payload.RealmID).Uint32("auctionID", payload.AuctionID).Msg("Auction expired, removed from cache via event")
}
