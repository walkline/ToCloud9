package grpcapi

import (
	"context"

	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
)

func (w *WorldServerGRPCAPI) CanPlayerInteractWithNPC(ctx context.Context, request *pb.CanPlayerInteractWithNPCRequest) (*pb.CanPlayerInteractWithNPCResponse, error) {
	if request.PlayerGuid == 0 {
		return &pb.CanPlayerInteractWithNPCResponse{
			Api: LibVer,
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		canInteract bool
		err         error
	}
	var resp respType

	respChan := make(chan respType, 1)

	w.readQueue.Push(queue.HandlerFunc(func() {
		canInteract, err := w.bindings.CanPlayerInteractWithNPC(request.PlayerGuid, request.NpcGuid, request.NpcFlags)
		respChan <- respType{
			canInteract: canInteract,
			err:         err,
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

	return &pb.CanPlayerInteractWithNPCResponse{
		Api:         LibVer,
		CanInteract: resp.canInteract,
	}, nil
}

func (w *WorldServerGRPCAPI) CanPlayerInteractWithGameObject(ctx context.Context, request *pb.CanPlayerInteractWithGameObjectRequest) (*pb.CanPlayerInteractWithGameObjectResponse, error) {
	if request.PlayerGuid == 0 {
		return &pb.CanPlayerInteractWithGameObjectResponse{
			Api: LibVer,
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		canInteract bool
		err         error
	}
	var resp respType

	respChan := make(chan respType, 1)

	w.readQueue.Push(queue.HandlerFunc(func() {
		canInteract, err := w.bindings.CanPlayerInteractWithGO(request.PlayerGuid, request.GameObjectGuid, uint8(request.GameObjectType))
		respChan <- respType{
			canInteract: canInteract,
			err:         err,
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

	return &pb.CanPlayerInteractWithGameObjectResponse{
		Api:         LibVer,
		CanInteract: resp.canInteract,
	}, nil
}
