package grpcapi

import (
	"context"
	"errors"
	"time"

	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
)

var ErrTimeout = errors.New("request timeouted")

type RequestQueue struct {
}

type WorldServerGRPCAPI struct {
	bindings   CppBindings
	timeout    time.Duration
	readQueue  queue.HandlersQueue
	writeQueue queue.HandlersQueue
}

func NewWorldServerGRPCAPI(bindings CppBindings, timeout time.Duration, readQueue, writeQueue queue.HandlersQueue) pb.WorldServerServiceServer {
	return &WorldServerGRPCAPI{
		bindings:   bindings,
		timeout:    timeout,
		readQueue:  readQueue,
		writeQueue: writeQueue,
	}
}

func (w *WorldServerGRPCAPI) GetPlayerItemsByGuids(ctx context.Context, request *pb.GetPlayerItemsByGuidsRequest) (*pb.GetPlayerItemsByGuidsResponse, error) {
	if request.PlayerGuid == 0 || len(request.Guids) == 0 {
		return &pb.GetPlayerItemsByGuidsResponse{
			Api: "0.0.1",
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
		Api:   "0.0.1",
		Items: items,
	}, nil
}

func (w *WorldServerGRPCAPI) RemoveItemsWithGuidsFromPlayer(ctx context.Context, request *pb.RemoveItemsWithGuidsFromPlayerRequest) (*pb.RemoveItemsWithGuidsFromPlayerResponse, error) {
	if request.PlayerGuid == 0 || len(request.Guids) == 0 {
		return &pb.RemoveItemsWithGuidsFromPlayerResponse{
			Api: "0.0.1",
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
		Api:               "0.0.1",
		UpdatedItemsGuids: resp.items,
	}, nil
}

func (w *WorldServerGRPCAPI) AddExistingItemToPlayer(ctx context.Context, request *pb.AddExistingItemToPlayerRequest) (*pb.AddExistingItemToPlayerResponse, error) {
	if request.PlayerGuid == 0 {
		return &pb.AddExistingItemToPlayerResponse{
			Api: "0.0.1",
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
				Api:    "0.0.1",
				Status: pb.AddExistingItemToPlayerResponse_NoSpace,
			}, nil
		}
		return nil, respErr
	}

	return &pb.AddExistingItemToPlayerResponse{
		Api:    "0.0.1",
		Status: pb.AddExistingItemToPlayerResponse_Success,
	}, nil
}
