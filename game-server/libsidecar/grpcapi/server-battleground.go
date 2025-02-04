package grpcapi

import (
	"context"

	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
)

func (w *WorldServerGRPCAPI) StartBattleground(ctx context.Context, request *pb.StartBattlegroundRequest) (*pb.StartBattlegroundResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		resp *BattlegroundStartResponse
		err  error
	}
	var resp respType

	respChan := make(chan respType, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		r, err := w.bindings.StartBattleground(BattlegroundStartRequest{
			BattlegroundTypeID:       uint8(request.BattlegroundTypeID),
			ArenaType:                uint8(request.ArenaType),
			IsRated:                  request.IsRated,
			MapID:                    request.MapID,
			BracketLvl:               uint8(request.BracketLvl),
			HordePlayerGUIDsToAdd:    request.PlayersToAddHorde,
			AlliancePlayerGUIDsToAdd: request.PlayersToAddAlliance,
		})
		respChan <- respType{
			resp: r,
			err:  err,
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

	return &pb.StartBattlegroundResponse{
		Api:              LibVer,
		InstanceID:       uint32(resp.resp.InstanceID),
		ClientInstanceID: uint32(resp.resp.InstanceClientID),
	}, nil
}

func (w *WorldServerGRPCAPI) AddPlayersToBattleground(ctx context.Context, request *pb.AddPlayersToBattlegroundRequest) (*pb.AddPlayersToBattlegroundResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		err error
	}
	var resp respType

	respChan := make(chan respType, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		err := w.bindings.AddPlayersToBattleground(BattlegroundAddPlayersRequest{
			BattlegroundTypeID:       uint8(request.BattlegroundTypeID),
			InstanceID:               uint64(request.InstanceID),
			HordePlayerGUIDsToAdd:    request.PlayersToAddHorde,
			AlliancePlayerGUIDsToAdd: request.PlayersToAddAlliance,
		})
		respChan <- respType{
			err: err,
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

	return &pb.AddPlayersToBattlegroundResponse{
		Api: LibVer,
	}, nil
}

// rpc CanPlayerJoinBattlegroundQueue(CanPlayerJoinBattlegroundQueueRequest) returns (CanPlayerJoinBattlegroundQueueResponse);
// rpc CanPlayerTeleportToBattleground(CanPlayerTeleportToBattlegroundRequest) returns (CanPlayerTeleportToBattlegroundResponse);
func (w *WorldServerGRPCAPI) CanPlayerJoinBattlegroundQueue(ctx context.Context, request *pb.CanPlayerJoinBattlegroundQueueRequest) (*pb.CanPlayerJoinBattlegroundQueueResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		err error
	}
	var resp respType

	respChan := make(chan respType, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		err := w.bindings.CanPlayerJoinBattlegroundQueue(request.PlayerGUID)
		respChan <- respType{
			err: err,
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

	return &pb.CanPlayerJoinBattlegroundQueueResponse{
		Api: LibVer,
	}, nil
}

func (w *WorldServerGRPCAPI) CanPlayerTeleportToBattleground(ctx context.Context, request *pb.CanPlayerTeleportToBattlegroundRequest) (*pb.CanPlayerTeleportToBattlegroundResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		err error
	}
	var resp respType

	respChan := make(chan respType, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		err := w.bindings.CanPlayerTeleportToBattleground(request.PlayerGUID)
		respChan <- respType{
			err: err,
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

	return &pb.CanPlayerTeleportToBattlegroundResponse{
		Api: LibVer,
	}, nil
}
