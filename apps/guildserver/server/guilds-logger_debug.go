package server

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/gen/guilds/pb"
)

// guildDebugLoggerMiddleware middleware that adds debug logs for pb.GuildServiceServer.
type guildDebugLoggerMiddleware struct {
	pb.UnimplementedGuildServiceServer
	realService pb.GuildServiceServer
	logger      zerolog.Logger
}

// NewGuildsDebugLoggerMiddleware returns middleware for pb.GuildServiceServer that logs requests for debug.
func NewGuildsDebugLoggerMiddleware(realService pb.GuildServiceServer, logger zerolog.Logger) pb.GuildServiceServer {
	return &guildDebugLoggerMiddleware{
		realService: realService,
		logger:      logger,
	}
}

func (g *guildDebugLoggerMiddleware) GetGuildInfo(ctx context.Context, params *pb.GetInfoParams) (res *pb.GetInfoResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("guildID", params.GuildID).
			Err(err).
			Msgf("Handled GetGuildInfo for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.GetGuildInfo(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) GetRosterInfo(ctx context.Context, params *pb.GetRosterInfoParams) (res *pb.GetRosterInfoResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("guildID", params.GuildID).
			Err(err).
			Msgf("Handled GetRosterInfo for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.GetRosterInfo(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) InviteMember(ctx context.Context, params *pb.InviteMemberParams) (res *pb.InviteMemberResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("invitee", params.Invitee).
			Uint64("inviter", params.Inviter).
			Err(err).
			Msgf("Handled InviteMember for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.InviteMember(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) InviteAccepted(ctx context.Context, params *pb.InviteAcceptedParams) (res *pb.InviteAcceptedResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("character", params.Character.Guid).
			Err(err).
			Msgf("Handled InviteAccepted for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.InviteAccepted(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) Leave(ctx context.Context, params *pb.LeaveParams) (res *pb.LeaveResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("leaver", params.Leaver).
			Err(err).
			Msgf("Handled Leave for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.Leave(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) Kick(ctx context.Context, params *pb.KickParams) (res *pb.KickResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("kicker", params.Kicker).
			Uint64("target", params.Target).
			Err(err).
			Msgf("Handled Kick for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.Kick(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) SetMessageOfTheDay(ctx context.Context, params *pb.SetMessageOfTheDayParams) (res *pb.SetMessageOfTheDayResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("updater", params.ChangerGUID).
			Err(err).
			Msgf("Handled SetMessageOfTheDay for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.SetMessageOfTheDay(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) SetGuildInfo(ctx context.Context, params *pb.SetGuildInfoParams) (res *pb.SetGuildInfoResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("updater", params.ChangerGUID).
			Err(err).
			Msgf("Handled SetGuildInfo for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.SetGuildInfo(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) SetMemberPublicNote(ctx context.Context, params *pb.SetNoteParams) (res *pb.SetNoteResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("updater", params.ChangerGUID).
			Uint64("target", params.TargetGUID).
			Err(err).
			Msgf("Handled SetMemberPublicNote for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.SetMemberPublicNote(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) SetMemberOfficerNote(ctx context.Context, params *pb.SetNoteParams) (res *pb.SetNoteResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("updater", params.ChangerGUID).
			Uint64("target", params.TargetGUID).
			Err(err).
			Msgf("Handled SetMemberOfficerNote for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.SetMemberOfficerNote(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) UpdateRank(ctx context.Context, params *pb.RankUpdateParams) (res *pb.RankUpdateResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("updater", params.ChangerGUID).
			Err(err).
			Msgf("Handled UpdateRank for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.UpdateRank(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) AddRank(ctx context.Context, params *pb.AddRankParams) (res *pb.AddRankResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("updater", params.ChangerGUID).
			Err(err).
			Msgf("Handled UpdateRank for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.AddRank(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) DeleteLastRank(ctx context.Context, params *pb.DeleteLastRankParams) (res *pb.DeleteLastRankResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("updater", params.ChangerGUID).
			Err(err).
			Msgf("Handled DeleteLastRank for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.DeleteLastRank(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) PromoteMember(ctx context.Context, params *pb.PromoteDemoteParams) (res *pb.PromoteDemoteResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("updater", params.ChangerGUID).
			Uint64("target", params.TargetGUID).
			Err(err).
			Msgf("Handled PromoteMember for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.PromoteMember(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) DemoteMember(ctx context.Context, params *pb.PromoteDemoteParams) (res *pb.PromoteDemoteResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("updater", params.ChangerGUID).
			Uint64("target", params.TargetGUID).
			Err(err).
			Msgf("Handled DemoteMember for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.DemoteMember(ctx, params)
	return
}

func (g *guildDebugLoggerMiddleware) SendGuildMessage(ctx context.Context, params *pb.SendGuildMessageParams) (res *pb.SendGuildMessageResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("sender", params.SenderGUID).
			Err(err).
			Msgf("Handled SendGuildMessage for %v.", time.Since(t))
	}(time.Now())

	res, err = g.realService.SendGuildMessage(ctx, params)
	return
}
