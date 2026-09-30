package server

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/walkline/ToCloud9/gen/matchmaking/pb"
)

// matchmakingDebugLoggerMiddleware middleware that adds debug logs for pb.MailServiceServer.
type matchmakingDebugLoggerMiddleware struct {
	pb.UnimplementedMatchmakingServiceServer
	mmServer pb.MatchmakingServiceServer
	logger   zerolog.Logger
}

// NewMatchmakingDebugLoggerMiddleware returns middleware for pb.MatchmakingServiceServer that logs requests for debug.
func NewMatchmakingDebugLoggerMiddleware(mailServer pb.MatchmakingServiceServer, logger zerolog.Logger) pb.MatchmakingServiceServer {
	return &matchmakingDebugLoggerMiddleware{
		mmServer: mailServer,
		logger:   logger,
	}
}

func (m *matchmakingDebugLoggerMiddleware) EnqueueToBattleground(ctx context.Context, params *pb.EnqueueToBattlegroundRequest) (res *pb.EnqueueToBattlegroundResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().
			Interface("payload", params).
			Err(err).
			Msgf("Handled EnqueueToBattleground for %v.", time.Since(t))
	}(time.Now())

	res, err = m.mmServer.EnqueueToBattleground(ctx, params)
	return
}

func (m *matchmakingDebugLoggerMiddleware) RemovePlayerFromQueue(ctx context.Context, params *pb.RemovePlayerFromQueueRequest) (res *pb.RemovePlayerFromQueueResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().
			Interface("payload", params).
			Err(err).
			Msgf("Handled RemovePlayerFromQueue for %v.", time.Since(t))
	}(time.Now())

	res, err = m.mmServer.RemovePlayerFromQueue(ctx, params)
	return
}

func (m *matchmakingDebugLoggerMiddleware) BattlegroundQueueDataForPlayer(ctx context.Context, params *pb.BattlegroundQueueDataForPlayerRequest) (res *pb.BattlegroundQueueDataForPlayerResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().
			Interface("payload", params).
			Interface("res", res).
			Err(err).
			Msgf("Handled BattlegroundQueueDataForPlayer for %v.", time.Since(t))
	}(time.Now())

	res, err = m.mmServer.BattlegroundQueueDataForPlayer(ctx, params)
	return
}

func (m *matchmakingDebugLoggerMiddleware) PlayerLeftBattleground(ctx context.Context, params *pb.PlayerLeftBattlegroundRequest) (res *pb.PlayerLeftBattlegroundResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().
			Interface("payload", params).
			Err(err).
			Msgf("Handled PlayerLeftBattleground for %v.", time.Since(t))
	}(time.Now())

	res, err = m.mmServer.PlayerLeftBattleground(ctx, params)
	return
}

func (m *matchmakingDebugLoggerMiddleware) PlayerJoinedBattleground(ctx context.Context, params *pb.PlayerJoinedBattlegroundRequest) (res *pb.PlayerJoinedBattlegroundResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().
			Interface("payload", params).
			Err(err).
			Msgf("Handled PlayerJoinedBattleground for %v.", time.Since(t))
	}(time.Now())

	res, err = m.mmServer.PlayerJoinedBattleground(ctx, params)
	return
}

func (m *matchmakingDebugLoggerMiddleware) BattlegroundStatusChanged(ctx context.Context, params *pb.BattlegroundStatusChangedRequest) (res *pb.BattlegroundStatusChangedResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().
			Interface("payload", params).
			Err(err).
			Msgf("Handled BattlegroundStatusChanged for %v.", time.Since(t))
	}(time.Now())

	res, err = m.mmServer.BattlegroundStatusChanged(ctx, params)
	return
}

func (m *matchmakingDebugLoggerMiddleware) JoinLFG(ctx context.Context, params *pb.JoinLFGRequest) (res *pb.JoinLFGResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().Interface("payload", params).Interface("res", res).Err(err).
			Msgf("Handled JoinLFG for %v.", time.Since(t))
	}(time.Now())
	return m.mmServer.JoinLFG(ctx, params)
}

func (m *matchmakingDebugLoggerMiddleware) LeaveLFG(ctx context.Context, params *pb.LeaveLFGRequest) (res *pb.LeaveLFGResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().Interface("payload", params).Err(err).
			Msgf("Handled LeaveLFG for %v.", time.Since(t))
	}(time.Now())
	return m.mmServer.LeaveLFG(ctx, params)
}

func (m *matchmakingDebugLoggerMiddleware) GetLFGStatus(ctx context.Context, params *pb.GetLFGStatusRequest) (res *pb.GetLFGStatusResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().Interface("payload", params).Interface("res", res).Err(err).
			Msgf("Handled GetLFGStatus for %v.", time.Since(t))
	}(time.Now())
	return m.mmServer.GetLFGStatus(ctx, params)
}
