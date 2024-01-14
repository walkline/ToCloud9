package server

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/gen/group/pb"
)

// groupDebugLoggerMiddleware middleware that adds debug logs for pb.GroupServiceServer.
type groupDebugLoggerMiddleware struct {
	pb.UnimplementedGroupServiceServer
	realServer pb.GroupServiceServer
	logger     zerolog.Logger
}

// NewGroupsDebugLoggerMiddleware returns middleware for pb.GroupServiceServer that logs requests for debug.
func NewGroupsDebugLoggerMiddleware(realService pb.GroupServiceServer, logger zerolog.Logger) pb.GroupServiceServer {
	return &groupDebugLoggerMiddleware{
		realServer: realService,
		logger:     logger,
	}
}

func (g groupDebugLoggerMiddleware) GetGroup(ctx context.Context, params *pb.GetGroupRequest) (res *pb.GetGroupResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint32("groupID", params.GroupID).
			Err(err).
			Msgf("Handled GetGroup for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.GetGroup(ctx, params)
	return
}

func (g groupDebugLoggerMiddleware) GetGroupIDByPlayer(ctx context.Context, params *pb.GetGroupIDByPlayerRequest) (res *pb.GetGroupIDByPlayerResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("player", params.Player).
			Err(err).
			Msgf("Handled GetGroupIDByPlayer for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.GetGroupIDByPlayer(ctx, params)
	return
}

func (g groupDebugLoggerMiddleware) Invite(ctx context.Context, params *pb.InviteParams) (res *pb.InviteResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("invited", params.Invited).
			Uint64("inviter", params.Inviter).
			Err(err).
			Msgf("Handled Invite for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.Invite(ctx, params)
	return
}

func (g groupDebugLoggerMiddleware) AcceptInvite(ctx context.Context, params *pb.AcceptInviteParams) (res *pb.AcceptInviteResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("player", params.Player).
			Err(err).
			Msgf("Handled AcceptInvite for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.AcceptInvite(ctx, params)
	return
}

func (g groupDebugLoggerMiddleware) Uninvite(ctx context.Context, params *pb.UninviteParams) (res *pb.UninviteResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("initiator", params.Initiator).
			Uint64("target", params.Target).
			Err(err).
			Msgf("Handled Uninvite for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.Uninvite(ctx, params)
	return
}

func (g groupDebugLoggerMiddleware) Leave(ctx context.Context, params *pb.GroupLeaveParams) (res *pb.GroupLeaveResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("player", params.Player).
			Err(err).
			Msgf("Handled Leave for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.Leave(ctx, params)
	return
}

func (g groupDebugLoggerMiddleware) ConvertToRaid(ctx context.Context, params *pb.ConvertToRaidParams) (res *pb.ConvertToRaidResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("player", params.Player).
			Err(err).
			Msgf("Handled ConvertToRaid for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.ConvertToRaid(ctx, params)
	return
}

func (g groupDebugLoggerMiddleware) ChangeLeader(ctx context.Context, params *pb.ChangeLeaderParams) (res *pb.ChangeLeaderResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("player", params.Player).
			Err(err).
			Msgf("Handled ChangeLeader for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.ChangeLeader(ctx, params)
	return
}

func (g groupDebugLoggerMiddleware) SendMessage(ctx context.Context, params *pb.SendGroupMessageParams) (res *pb.SendGroupMessageResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("sender", params.SenderGUID).
			Uint32("msgType", params.MessageType).
			Err(err).
			Msgf("Handled SendMessage for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.SendMessage(ctx, params)
	return
}

func (g groupDebugLoggerMiddleware) GetGroupByMember(ctx context.Context, params *pb.GetGroupByMemberRequest) (res *pb.GetGroupResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("player", params.Player).
			Err(err).
			Msgf("Handled GetGroupByMember for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.GetGroupByMember(ctx, params)
	return
}

func (g groupDebugLoggerMiddleware) SetGroupTargetIcon(ctx context.Context, params *pb.SetGroupTargetIconRequest) (res *pb.SetGroupTargetIconResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("setter", params.SetterGUID).
			Uint64("target", params.TargetGUID).
			Uint32("iconID", params.IconID).
			Err(err).
			Msgf("Handled SetGroupTargetIcon for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.SetGroupTargetIcon(ctx, params)
	return
}

func (g groupDebugLoggerMiddleware) SetLootMethod(ctx context.Context, params *pb.SetLootMethodRequest) (res *pb.SetLootMethodResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("updater", params.PlayerGUID).
			Uint32("method", params.Method).
			Err(err).
			Msgf("Handled SetLootMethod for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.SetLootMethod(ctx, params)
	return
}

func (g groupDebugLoggerMiddleware) SetDungeonDifficulty(ctx context.Context, params *pb.SetDungeonDifficultyRequest) (res *pb.SetDungeonDifficultyResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("updater", params.PlayerGUID).
			Uint32("difficulty", params.Difficulty).
			Err(err).
			Msgf("Handled SetDungeonDifficulty for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.SetDungeonDifficulty(ctx, params)
	return
}

func (g groupDebugLoggerMiddleware) SetRaidDifficulty(ctx context.Context, params *pb.SetRaidDifficultyRequest) (res *pb.SetRaidDifficultyResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("updater", params.PlayerGUID).
			Uint32("difficulty", params.Difficulty).
			Err(err).
			Msgf("Handled SetRaidDifficulty for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realServer.SetRaidDifficulty(ctx, params)
	return
}
