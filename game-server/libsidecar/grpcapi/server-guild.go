package grpcapi

import (
	"context"

	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
)

func (w *WorldServerGRPCAPI) SetPlayerGuildFields(ctx context.Context, request *pb.SetPlayerGuildFieldsRequest) (*pb.SetPlayerGuildFieldsResponse, error) {
	if request.PlayerGuid == 0 {
		return &pb.SetPlayerGuildFieldsResponse{
			Api: LibVer,
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		applied bool
		err     error
	}
	var resp respType

	respChan := make(chan respType, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		applied, err := w.bindings.SetPlayerGuildFields(request.PlayerGuid, request.GuildID, request.Rank)
		respChan <- respType{applied: applied, err: err}
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

	return &pb.SetPlayerGuildFieldsResponse{
		Api:     LibVer,
		Applied: resp.applied,
	}, nil
}
