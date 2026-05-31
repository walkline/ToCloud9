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

func (m *matchmakingDebugLoggerMiddleware) FinishRatedArenaMatch(ctx context.Context, params *pb.FinishRatedArenaMatchRequest) (res *pb.FinishRatedArenaMatchResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().
			Interface("payload", params).
			Interface("res", res).
			Err(err).
			Msgf("Handled FinishRatedArenaMatch for %v.", time.Since(t))
	}(time.Now())

	res, err = m.mmServer.FinishRatedArenaMatch(ctx, params)
	return
}

func (m *matchmakingDebugLoggerMiddleware) JoinLfg(ctx context.Context, params *pb.JoinLfgRequest) (res *pb.JoinLfgResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().
			Interface("payload", params).
			Interface("res", res).
			Err(err).
			Msgf("Handled JoinLfg for %v.", time.Since(t))
	}(time.Now())

	res, err = m.mmServer.JoinLfg(ctx, params)
	return
}

func (m *matchmakingDebugLoggerMiddleware) LeaveLfg(ctx context.Context, params *pb.LeaveLfgRequest) (res *pb.LeaveLfgResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().
			Interface("payload", params).
			Err(err).
			Msgf("Handled LeaveLfg for %v.", time.Since(t))
	}(time.Now())

	res, err = m.mmServer.LeaveLfg(ctx, params)
	return
}

func (m *matchmakingDebugLoggerMiddleware) SetLfgRoles(ctx context.Context, params *pb.SetLfgRolesRequest) (res *pb.SetLfgRolesResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().
			Interface("payload", params).
			Interface("res", res).
			Err(err).
			Msgf("Handled SetLfgRoles for %v.", time.Since(t))
	}(time.Now())

	res, err = m.mmServer.SetLfgRoles(ctx, params)
	return
}

func (m *matchmakingDebugLoggerMiddleware) AnswerLfgProposal(ctx context.Context, params *pb.AnswerLfgProposalRequest) (res *pb.AnswerLfgProposalResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().
			Interface("payload", params).
			Interface("res", res).
			Err(err).
			Msgf("Handled AnswerLfgProposal for %v.", time.Since(t))
	}(time.Now())

	res, err = m.mmServer.AnswerLfgProposal(ctx, params)
	return
}

func (m *matchmakingDebugLoggerMiddleware) LfgStatus(ctx context.Context, params *pb.LfgStatusRequest) (res *pb.LfgStatusResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().
			Interface("payload", params).
			Interface("res", res).
			Err(err).
			Msgf("Handled LfgStatus for %v.", time.Since(t))
	}(time.Now())

	res, err = m.mmServer.LfgStatus(ctx, params)
	return
}

func (m *matchmakingDebugLoggerMiddleware) CompleteLfgDungeon(ctx context.Context, params *pb.CompleteLfgDungeonRequest) (res *pb.CompleteLfgDungeonResponse, err error) {
	defer func(t time.Time) {
		m.logger.Debug().
			Interface("payload", params).
			Err(err).
			Msgf("Handled CompleteLfgDungeon for %v.", time.Since(t))
	}(time.Now())

	res, err = m.mmServer.CompleteLfgDungeon(ctx, params)
	return
}
