package server

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/walkline/ToCloud9/apps/auctionhouse/repo"
	"github.com/walkline/ToCloud9/apps/auctionhouse/service"
	"github.com/walkline/ToCloud9/gen/auctionhouse/pb"
)

type AuctionHouseServer struct {
	pb.UnimplementedAuctionHouseServiceServer
	service *service.AuctionService
}

func NewAuctionHouseServer(svc *service.AuctionService) pb.AuctionHouseServiceServer {
	return &AuctionHouseServer{service: svc}
}

func (s *AuctionHouseServer) Hello(_ context.Context, req *pb.AuctionHelloRequest) (*pb.AuctionHelloResponse, error) {
	return &pb.AuctionHelloResponse{
		HouseID: req.HouseID,
	}, nil
}

func (s *AuctionHouseServer) SellItem(ctx context.Context, req *pb.AuctionSellItemRequest) (*pb.AuctionSellItemResponse, error) {
	entry := &repo.AuctionEntry{
		HouseID:          uint8(req.HouseID),
		ItemGUID:         uint32(req.ItemGuid),
		BuyoutPrice:      req.Buyout,
		Time:             uint32(time.Now().Unix()) + req.ExpireTimeSecs,
		BuyGUID:          0,
		LastBid:          0,
		StartBid:         req.StartBid,
		Deposit:          req.Deposit,
		ItemEntry:        req.ItemEntry,
		ItemCount:        req.ItemCount,
		Charges:          req.Charges,
		RandomPropertyID: int32(req.RandomPropertyID),
		SuffixFactor:     req.SuffixFactor,
		Flags:            req.Flags,
	}

	enchStrs := make([]string, 0)
	for _, e := range req.Enchantments {
		enchStrs = append(enchStrs, "")
		_ = e
	}
	entry.Enchantments = strings.Join(enchStrs, " ")

	auctionID, err := s.service.SellItem(ctx, req.RealmID, req.PlayerGuid, entry)
	if err != nil {
		return &pb.AuctionSellItemResponse{
			Error: pb.AuctionHouseError_AH_DATABASE_ERROR,
		}, nil
	}

	return &pb.AuctionSellItemResponse{
		Error:     pb.AuctionHouseError_AH_OK,
		AuctionID: auctionID,
	}, nil
}

func (s *AuctionHouseServer) PlaceBid(ctx context.Context, req *pb.AuctionPlaceBidRequest) (*pb.AuctionPlaceBidResponse, error) {
	isBuyout, moneyToDeduct, err := s.service.PlaceBid(ctx, req.RealmID, req.PlayerGuid, req.AuctionID, req.Price)
	if err != nil {
		ahErr := pb.AuctionHouseError_AH_DATABASE_ERROR
		switch {
		case errors.Is(err, service.ErrAuctionNotFound):
			ahErr = pb.AuctionHouseError_AH_ITEM_NOT_FOUND
		case errors.Is(err, service.ErrBidOwnAuction):
			ahErr = pb.AuctionHouseError_AH_BID_OWN
		case errors.Is(err, service.ErrBidTooLow):
			ahErr = pb.AuctionHouseError_AH_HIGHER_BID
		case errors.Is(err, service.ErrBidIncrementTooLow):
			ahErr = pb.AuctionHouseError_AH_BID_INCREMENT
		}
		return &pb.AuctionPlaceBidResponse{
			Error: ahErr,
		}, nil
	}

	return &pb.AuctionPlaceBidResponse{
		Error:          pb.AuctionHouseError_AH_OK,
		AuctionID:      req.AuctionID,
		IsBuyout:       isBuyout,
		MoneyToDeduct:  moneyToDeduct,
	}, nil
}

func (s *AuctionHouseServer) CancelAuction(ctx context.Context, req *pb.AuctionCancelRequest) (*pb.AuctionCancelResponse, error) {
	auctionCut, err := s.service.CancelAuction(ctx, req.RealmID, req.PlayerGuid, req.AuctionID)
	if err != nil {
		return &pb.AuctionCancelResponse{
			Error: pb.AuctionHouseError_AH_DATABASE_ERROR,
		}, nil
	}

	return &pb.AuctionCancelResponse{
		Error:      pb.AuctionHouseError_AH_OK,
		AuctionID:  req.AuctionID,
		AuctionCut: auctionCut,
	}, nil
}

func (s *AuctionHouseServer) ListItems(_ context.Context, req *pb.AuctionListItemsRequest) (*pb.AuctionListItemsResponse, error) {
	sorting := make([]service.AuctionSortInfo, len(req.Sorting))
	for i, so := range req.Sorting {
		sorting[i] = service.AuctionSortInfo{Column: so.Column, IsDesc: so.IsDesc}
	}

	items, totalCount := s.service.ListItems(
		req.RealmID, req.HouseID, req.ListFrom, req.SearchedName,
		req.LevelMin, req.LevelMax, req.InventoryType,
		req.ItemClass, req.ItemSubClass, req.Quality,
		req.GetAll, sorting,
	)

	return &pb.AuctionListItemsResponse{
		Items:       cachedAuctionsToProto(items),
		TotalCount:  totalCount,
		SearchDelay: service.AuctionSearchDelay,
	}, nil
}

func (s *AuctionHouseServer) ListOwnerItems(_ context.Context, req *pb.AuctionListOwnerItemsRequest) (*pb.AuctionListOwnerItemsResponse, error) {
	items := s.service.ListOwnerItems(req.RealmID, req.PlayerGuid, req.HouseID)

	return &pb.AuctionListOwnerItemsResponse{
		Items:       cachedAuctionsToProto(items),
		TotalCount:  uint32(len(items)),
		SearchDelay: service.AuctionSearchDelay,
	}, nil
}

func (s *AuctionHouseServer) ListBidderItems(_ context.Context, req *pb.AuctionListBidderItemsRequest) (*pb.AuctionListBidderItemsResponse, error) {
	items := s.service.ListBidderItems(req.RealmID, req.PlayerGuid, req.HouseID, req.OutbiddedAuctionIDs)

	return &pb.AuctionListBidderItemsResponse{
		Items:       cachedAuctionsToProto(items),
		TotalCount:  uint32(len(items)),
		SearchDelay: service.AuctionSearchDelay,
	}, nil
}

func (s *AuctionHouseServer) ListPendingSales(_ context.Context, _ *pb.AuctionListPendingSalesRequest) (*pb.AuctionListPendingSalesResponse, error) {
	return &pb.AuctionListPendingSalesResponse{
		Count: 0,
	}, nil
}

func cachedAuctionsToProto(auctions []*service.CachedAuction) []*pb.AuctionItem {
	result := make([]*pb.AuctionItem, len(auctions))
	for i, a := range auctions {
		result[i] = &pb.AuctionItem{
			AuctionID:        a.ID,
			ItemEntry:        a.ItemEntry,
			ItemCount:        a.ItemCount,
			Charges:          a.Charges,
			RandomPropertyID: a.RandomPropertyID,
			SuffixFactor:     a.SuffixFactor,
			ItemGuid:         uint64(a.ItemGUID),
			OwnerGuid:        uint64(a.ItemOwner),
			StartBid:         a.StartBid,
			Bid:              a.LastBid,
			Buyout:           a.BuyoutPrice,
			ExpireTime:       int64(a.Time),
			BidderGuid:       uint64(a.BuyGUID),
			Deposit:          a.Deposit,
			HouseID:          uint32(a.HouseID),
			Flags:            a.Flags,
		}
	}
	return result
}