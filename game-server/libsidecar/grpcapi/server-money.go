package grpcapi

import (
	"context"

	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
)

func (w *WorldServerGRPCAPI) GetMoneyForPlayer(ctx context.Context, request *pb.GetMoneyForPlayerRequest) (*pb.GetMoneyForPlayerResponse, error) {
	if request.PlayerGuid == 0 {
		return &pb.GetMoneyForPlayerResponse{
			Api: LibVer,
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		money uint32
		err   error
	}
	var resp respType

	respChan := make(chan respType, 1)

	w.readQueue.Push(queue.HandlerFunc(func() {
		money, err := w.bindings.GetMoneyForPlayer(request.PlayerGuid)
		respChan <- respType{
			money: money,
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

	return &pb.GetMoneyForPlayerResponse{
		Api:   LibVer,
		Money: resp.money,
	}, nil
}

func (w *WorldServerGRPCAPI) ModifyMoneyForPlayer(ctx context.Context, request *pb.ModifyMoneyForPlayerRequest) (*pb.ModifyMoneyForPlayerResponse, error) {
	if request.PlayerGuid == 0 {
		return &pb.ModifyMoneyForPlayerResponse{
			Api: LibVer,
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		money uint32
		err   error
	}
	var resp respType

	respChan := make(chan respType, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		money, err := w.bindings.ModifyMoneyForPlayer(request.PlayerGuid, request.Value)
		respChan <- respType{
			money: money,
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

	return &pb.ModifyMoneyForPlayerResponse{
		Api:           LibVer,
		NewMoneyValue: resp.money,
	}, nil
}
