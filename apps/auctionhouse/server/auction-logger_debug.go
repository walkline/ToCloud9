package server

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/gen/auctionhouse/pb"
)

// auctionHouseDebugLoggerMiddleware middleware that adds debug logs for pb.AuctionHouseServiceServer.
type auctionHouseDebugLoggerMiddleware struct {
	pb.UnimplementedAuctionHouseServiceServer
	realService pb.AuctionHouseServiceServer
	logger      zerolog.Logger
}

// NewAuctionHouseDebugLoggerMiddleware returns middleware for pb.AuctionHouseServiceServer that logs requests for debug.
func NewAuctionHouseDebugLoggerMiddleware(realService pb.AuctionHouseServiceServer, logger zerolog.Logger) pb.AuctionHouseServiceServer {
	return &auctionHouseDebugLoggerMiddleware{
		realService: realService,
		logger:      logger,
	}
}

func (m *auctionHouseDebugLoggerMiddleware) Hello(ctx context.Context, req *pb.AuctionHelloRequest) (resp *pb.AuctionHelloResponse, err error) {
	defer func(t time.Time) {
		event := m.logger.Debug().
			Uint32("realmID", req.RealmID).
			Uint32("houseID", req.HouseID).
			Str("timeTook", time.Since(t).String())

		if resp != nil {
			event = event.Uint32("returnedHouseID", resp.HouseID)
		}

		event.Msg("Handled Hello")
	}(time.Now())

	resp, err = m.realService.Hello(ctx, req)
	return
}

func (m *auctionHouseDebugLoggerMiddleware) SellItem(ctx context.Context, req *pb.AuctionSellItemRequest) (resp *pb.AuctionSellItemResponse, err error) {
	defer func(t time.Time) {
		event := m.logger.Debug().
			Uint32("realmID", req.RealmID).
			Uint64("playerGuid", req.PlayerGuid).
			Uint32("houseID", req.HouseID).
			Uint32("itemEntry", req.ItemEntry).
			Uint64("itemGuid", req.ItemGuid).
			Uint32("itemCount", req.ItemCount).
			Int32("charges", req.Charges).
			Uint32("startBid", req.StartBid).
			Uint32("buyout", req.Buyout).
			Uint32("expireTimeSecs", req.ExpireTimeSecs).
			Uint32("deposit", req.Deposit).
			Str("timeTook", time.Since(t).String())

		if resp != nil {
			event = event.
				Str("error", resp.Error.String()).
				Uint32("auctionID", resp.AuctionID)
		}

		event.Msg("Handled SellItem")
	}(time.Now())

	resp, err = m.realService.SellItem(ctx, req)
	return
}

func (m *auctionHouseDebugLoggerMiddleware) PlaceBid(ctx context.Context, req *pb.AuctionPlaceBidRequest) (resp *pb.AuctionPlaceBidResponse, err error) {
	defer func(t time.Time) {
		event := m.logger.Debug().
			Uint32("realmID", req.RealmID).
			Uint64("playerGuid", req.PlayerGuid).
			Uint32("houseID", req.HouseID).
			Uint32("auctionID", req.AuctionID).
			Uint32("price", req.Price).
			Str("timeTook", time.Since(t).String())

		if resp != nil {
			event = event.
				Str("error", resp.Error.String()).
				Bool("isBuyout", resp.IsBuyout).
				Uint32("moneyToDeduct", resp.MoneyToDeduct)
		}

		event.Msg("Handled PlaceBid")
	}(time.Now())

	resp, err = m.realService.PlaceBid(ctx, req)
	return
}

func (m *auctionHouseDebugLoggerMiddleware) CancelAuction(ctx context.Context, req *pb.AuctionCancelRequest) (resp *pb.AuctionCancelResponse, err error) {
	defer func(t time.Time) {
		event := m.logger.Debug().
			Uint32("realmID", req.RealmID).
			Uint64("playerGuid", req.PlayerGuid).
			Uint32("houseID", req.HouseID).
			Uint32("auctionID", req.AuctionID).
			Str("timeTook", time.Since(t).String())

		if resp != nil {
			event = event.
				Str("error", resp.Error.String()).
				Uint32("auctionCut", resp.AuctionCut)
		}

		event.Msg("Handled CancelAuction")
	}(time.Now())

	resp, err = m.realService.CancelAuction(ctx, req)
	return
}

func (m *auctionHouseDebugLoggerMiddleware) ListItems(ctx context.Context, req *pb.AuctionListItemsRequest) (resp *pb.AuctionListItemsResponse, err error) {
	defer func(t time.Time) {
		event := m.logger.Debug().
			Uint32("realmID", req.RealmID).
			Uint64("playerGuid", req.PlayerGuid).
			Uint32("houseID", req.HouseID).
			Uint32("listFrom", req.ListFrom).
			Str("searchedName", req.SearchedName).
			Uint32("levelMin", req.LevelMin).
			Uint32("levelMax", req.LevelMax).
			Uint32("inventoryType", req.InventoryType).
			Uint32("itemClass", req.ItemClass).
			Uint32("itemSubClass", req.ItemSubClass).
			Uint32("quality", req.Quality).
			Bool("getAll", req.GetAll).
			Bool("usable", req.Usable).
			Str("timeTook", time.Since(t).String())

		if resp != nil {
			event = event.
				Int("itemCount", len(resp.Items)).
				Uint32("totalCount", resp.TotalCount).
				Uint32("searchDelay", resp.SearchDelay)
		}

		event.Msg("Handled ListItems")
	}(time.Now())

	resp, err = m.realService.ListItems(ctx, req)
	return
}

func (m *auctionHouseDebugLoggerMiddleware) ListOwnerItems(ctx context.Context, req *pb.AuctionListOwnerItemsRequest) (resp *pb.AuctionListOwnerItemsResponse, err error) {
	defer func(t time.Time) {
		event := m.logger.Debug().
			Uint32("realmID", req.RealmID).
			Uint64("playerGuid", req.PlayerGuid).
			Uint32("houseID", req.HouseID).
			Str("timeTook", time.Since(t).String())

		if resp != nil {
			event = event.
				Int("itemCount", len(resp.Items)).
				Uint32("totalCount", resp.TotalCount).
				Uint32("searchDelay", resp.SearchDelay)
		}

		event.Msg("Handled ListOwnerItems")
	}(time.Now())

	resp, err = m.realService.ListOwnerItems(ctx, req)
	return
}

func (m *auctionHouseDebugLoggerMiddleware) ListBidderItems(ctx context.Context, req *pb.AuctionListBidderItemsRequest) (resp *pb.AuctionListBidderItemsResponse, err error) {
	defer func(t time.Time) {
		event := m.logger.Debug().
			Uint32("realmID", req.RealmID).
			Uint64("playerGuid", req.PlayerGuid).
			Uint32("houseID", req.HouseID).
			Int("outbiddedCount", len(req.OutbiddedAuctionIDs)).
			Str("timeTook", time.Since(t).String())

		if resp != nil {
			event = event.
				Int("itemCount", len(resp.Items)).
				Uint32("totalCount", resp.TotalCount).
				Uint32("searchDelay", resp.SearchDelay)
		}

		event.Msg("Handled ListBidderItems")
	}(time.Now())

	resp, err = m.realService.ListBidderItems(ctx, req)
	return
}

func (m *auctionHouseDebugLoggerMiddleware) ListPendingSales(ctx context.Context, req *pb.AuctionListPendingSalesRequest) (resp *pb.AuctionListPendingSalesResponse, err error) {
	defer func(t time.Time) {
		event := m.logger.Debug().
			Uint32("realmID", req.RealmID).
			Uint64("playerGuid", req.PlayerGuid).
			Str("timeTook", time.Since(t).String())

		if resp != nil {
			event = event.Uint32("count", resp.Count)
		}

		event.Msg("Handled ListPendingSales")
	}(time.Now())

	resp, err = m.realService.ListPendingSales(ctx, req)
	return
}
