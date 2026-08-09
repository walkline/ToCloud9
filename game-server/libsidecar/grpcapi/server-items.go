package grpcapi

import (
	"context"

	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
)

func (w *WorldServerGRPCAPI) GetPlayerItemsByGuids(ctx context.Context, request *pb.GetPlayerItemsByGuidsRequest) (*pb.GetPlayerItemsByGuidsResponse, error) {
	if request.PlayerGuid == 0 || len(request.Guids) == 0 {
		return &pb.GetPlayerItemsByGuidsResponse{
			Api: LibVer,
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		items []PlayerItem
		err   error
	}
	var resp respType

	respChan := make(chan respType, 1)

	w.readQueue.Push(queue.HandlerFunc(func() {
		items, err := w.bindings.GetPlayerItemsByGuids(request.PlayerGuid, request.Guids)
		respChan <- respType{
			items: items,
			err:   err,
		}
		close(respChan)
	}))

	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case resp = <-respChan:
	}

	if resp.err != nil {
		return nil, resp.err
	}

	items := make([]*pb.GetPlayerItemsByGuidsResponse_Item, len(resp.items))
	for i, item := range resp.items {
		items[i] = &pb.GetPlayerItemsByGuidsResponse_Item{
			Guid:             item.Guid,
			Entry:            item.Entry,
			Owner:            item.Owner,
			BagSlot:          uint32(item.BagSlot),
			Slot:             uint32(item.Slot),
			IsTradable:       item.IsTradable,
			Count:            item.Count,
			Flags:            uint32(item.Flags),
			Durability:       item.Durability,
			RandomPropertyID: item.RandomPropertyID,
			Text:             item.Text,
		}
	}

	return &pb.GetPlayerItemsByGuidsResponse{
		Api:   LibVer,
		Items: items,
	}, nil
}

func (w *WorldServerGRPCAPI) RemoveItemsWithGuidsFromPlayer(ctx context.Context, request *pb.RemoveItemsWithGuidsFromPlayerRequest) (*pb.RemoveItemsWithGuidsFromPlayerResponse, error) {
	if request.PlayerGuid == 0 || len(request.Guids) == 0 {
		return &pb.RemoveItemsWithGuidsFromPlayerResponse{
			Api: LibVer,
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		items []uint64
		err   error
	}
	var resp respType

	respChan := make(chan respType, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		items, err := w.bindings.RemoveItemsWithGuidsFromPlayer(request.PlayerGuid, request.Guids, request.AssignToPlayerGuid)
		respChan <- respType{
			items: items,
			err:   err,
		}
		close(respChan)
	}))

	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case resp = <-respChan:
	}

	if resp.err != nil {
		return nil, resp.err
	}

	return &pb.RemoveItemsWithGuidsFromPlayerResponse{
		Api:               LibVer,
		UpdatedItemsGuids: resp.items,
	}, nil
}

func (w *WorldServerGRPCAPI) DestroyItemsWithGuidsFromPlayer(ctx context.Context, request *pb.DestroyItemsWithGuidsFromPlayerRequest) (*pb.DestroyItemsWithGuidsFromPlayerResponse, error) {
	if request.PlayerGuid == 0 || len(request.Guids) == 0 {
		return &pb.DestroyItemsWithGuidsFromPlayerResponse{
			Api: LibVer,
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		items []uint64
		err   error
	}
	var resp respType

	respChan := make(chan respType, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		items, err := w.bindings.DestroyItemsWithGuidsFromPlayer(request.PlayerGuid, request.Guids)
		respChan <- respType{
			items: items,
			err:   err,
		}
		close(respChan)
	}))

	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case resp = <-respChan:
	}

	if resp.err != nil {
		return nil, resp.err
	}

	return &pb.DestroyItemsWithGuidsFromPlayerResponse{
		Api:                  LibVer,
		DestroyedItemsGuids:  resp.items,
	}, nil
}

func (w *WorldServerGRPCAPI) AddExistingItemToPlayer(ctx context.Context, request *pb.AddExistingItemToPlayerRequest) (*pb.AddExistingItemToPlayerResponse, error) {
	if request.PlayerGuid == 0 {
		return &pb.AddExistingItemToPlayerResponse{
			Api: LibVer,
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	var respErr error

	respChan := make(chan error, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		respChan <- w.bindings.AddExistingItemToPlayer(request.PlayerGuid, &ItemToAdd{
			Guid:             request.Item.Guid,
			Entry:            request.Item.Entry,
			Count:            request.Item.Count,
			Flags:            uint16(request.Item.Flags),
			Durability:       request.Item.Durability,
			RandomPropertyID: request.Item.RandomPropertyID,
			Text:             request.Item.Text,
		})
		close(respChan)
	}))
	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case respErr = <-respChan:
	}

	if respErr != nil {
		if itemErr, ok := respErr.(ItemError); ok && itemErr == ItemErrorNoInventorySpace {
			return &pb.AddExistingItemToPlayerResponse{
				Api:    LibVer,
				Status: pb.AddExistingItemToPlayerResponse_NoSpace,
			}, nil
		}
		return nil, respErr
	}

	return &pb.AddExistingItemToPlayerResponse{
		Api:    LibVer,
		Status: pb.AddExistingItemToPlayerResponse_Success,
	}, nil
}

func (w *WorldServerGRPCAPI) StoreNewItem(ctx context.Context, request *pb.StoreNewItemRequest) (*pb.StoreNewItemResponse, error) {
	if request.PlayerGuid == 0 || request.ItemEntry == 0 {
		return &pb.StoreNewItemResponse{Api: LibVer, Status: pb.StoreNewItemResponse_Failed}, nil
	}
	count := request.Count
	if count == 0 {
		count = 1
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		guid uint64
		err  error
	}
	respChan := make(chan respType, 1)
	w.writeQueue.Push(queue.HandlerFunc(func() {
		g, err := w.bindings.StoreNewItem(request.PlayerGuid, request.ItemEntry, count, request.EnchantmentIDs)
		respChan <- respType{guid: g, err: err}
		close(respChan)
	}))

	var resp respType
	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case resp = <-respChan:
	}

	if resp.err != nil {
		if itemErr, ok := resp.err.(ItemError); ok {
			switch itemErr {
			case ItemErrorNoPlayer:
				return &pb.StoreNewItemResponse{Api: LibVer, Status: pb.StoreNewItemResponse_PlayerNotFound}, nil
			case ItemErrorNoInventorySpace:
				return &pb.StoreNewItemResponse{Api: LibVer, Status: pb.StoreNewItemResponse_NoSpace}, nil
			case ItemErrorUnknownTemplate:
				return &pb.StoreNewItemResponse{Api: LibVer, Status: pb.StoreNewItemResponse_UnknownTemplate}, nil
			case ItemErrorFailedToCreateItem:
				return &pb.StoreNewItemResponse{Api: LibVer, Status: pb.StoreNewItemResponse_Failed}, nil
			}
		}
		return nil, resp.err
	}

	return &pb.StoreNewItemResponse{
		Api:      LibVer,
		Status:   pb.StoreNewItemResponse_Ok,
		ItemGuid: resp.guid,
	}, nil
}

func (w *WorldServerGRPCAPI) SetItemPermanentEnchantment(ctx context.Context, request *pb.SetItemPermanentEnchantmentRequest) (*pb.SetItemPermanentEnchantmentResponse, error) {
	if request.PlayerGuid == 0 || request.ItemGuid == 0 {
		return &pb.SetItemPermanentEnchantmentResponse{Api: LibVer, Status: pb.SetItemPermanentEnchantmentResponse_Failed}, nil
	}
	if w.bindings.SetItemPermanentEnchantment == nil {
		return &pb.SetItemPermanentEnchantmentResponse{Api: LibVer, Status: pb.SetItemPermanentEnchantmentResponse_Failed}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		err error
	}
	respChan := make(chan respType, 1)
	w.writeQueue.Push(queue.HandlerFunc(func() {
		err := w.bindings.SetItemPermanentEnchantment(request.PlayerGuid, request.ItemGuid, request.Slot, request.EnchantmentId)
		respChan <- respType{err: err}
		close(respChan)
	}))

	var resp respType
	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case resp = <-respChan:
	}

	if resp.err != nil {
		if itemErr, ok := resp.err.(ItemError); ok {
			switch itemErr {
			case ItemErrorNoPlayer:
				return &pb.SetItemPermanentEnchantmentResponse{Api: LibVer, Status: pb.SetItemPermanentEnchantmentResponse_PlayerNotFound}, nil
			case ItemErrorItemNotFound:
				return &pb.SetItemPermanentEnchantmentResponse{Api: LibVer, Status: pb.SetItemPermanentEnchantmentResponse_ItemNotFound}, nil
			default:
				return &pb.SetItemPermanentEnchantmentResponse{Api: LibVer, Status: pb.SetItemPermanentEnchantmentResponse_Failed}, nil
			}
		}
		return nil, resp.err
	}

	return &pb.SetItemPermanentEnchantmentResponse{Api: LibVer, Status: pb.SetItemPermanentEnchantmentResponse_Ok}, nil
}
