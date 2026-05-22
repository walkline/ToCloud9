package grpcapi

import (
	"context"

	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
)

func (w *WorldServerGRPCAPI) CreateGuild(ctx context.Context, request *pb.CreateGuildRequest) (*pb.CreateGuildResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		resp *GuildCreateResponse
		err  error
	}

	respChan := make(chan respType, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		resp, err := w.bindings.CreateGuild(request.LeaderGuid, request.GuildName)
		respChan <- respType{resp: resp, err: err}
		close(respChan)
	}))

	var resp respType

	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case resp = <-respChan:
	}

	if resp.err != nil {
		return nil, resp.err
	}

	status := pb.CreateGuildResponse_Success

	if resp.resp == nil {
		status = pb.CreateGuildResponse_InternalError
	} else {
		switch resp.resp.ErrorCode {
		case 0:
			status = pb.CreateGuildResponse_Success
		case 1:
			status = pb.CreateGuildResponse_InternalError
		case 2:
			status = pb.CreateGuildResponse_NameExists
		case 3:
			status = pb.CreateGuildResponse_InvalidName
		case 4:
			status = pb.CreateGuildResponse_LeaderNotFound
		case 5:
			status = pb.CreateGuildResponse_InternalError
		default:
			status = pb.CreateGuildResponse_InternalError
		}
	}

	guildID := uint64(0)
	if resp.resp != nil {
		guildID = resp.resp.GuildID
	}

	return &pb.CreateGuildResponse{
		Api:     LibVer,
		Status:  status,
		GuildId: guildID,
	}, nil
}
