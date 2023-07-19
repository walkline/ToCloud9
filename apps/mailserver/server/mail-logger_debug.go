package server

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/gen/mail/pb"
)

// mailDebugLoggerMiddleware middleware that adds debug logs for pb.MailServiceServer.
type mailDebugLoggerMiddleware struct {
	mailServer pb.MailServiceServer
	logger     zerolog.Logger
}

// NewMailDebugLoggerMiddleware returns middleware for pb.MailServiceServer that logs requests for debug.
func NewMailDebugLoggerMiddleware(mailServer pb.MailServiceServer, logger zerolog.Logger) pb.MailServiceServer {
	return &mailDebugLoggerMiddleware{
		mailServer: mailServer,
		logger:     logger,
	}
}

func (g *mailDebugLoggerMiddleware) Send(ctx context.Context, params *pb.SendRequest) (res *pb.SendResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("receiverGUID", params.ReceiverGuid).
			Err(err).
			Msgf("Handled Send for %v.", time.Since(t))
	}(time.Now())

	res, err = g.mailServer.Send(ctx, params)
	return
}

func (g *mailDebugLoggerMiddleware) MailsForPlayer(ctx context.Context, params *pb.MailsForPlayerRequest) (res *pb.MailsForPlayerResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("playerGUID", params.PlayerGuid).
			Err(err).
			Msgf("Handled MailsForPlayer for %v.", time.Since(t))
	}(time.Now())

	res, err = g.mailServer.MailsForPlayer(ctx, params)
	return
}

func (g *mailDebugLoggerMiddleware) MarkAsReadForPlayer(ctx context.Context, params *pb.MarkAsReadForPlayerRequest) (res *pb.MarkAsReadForPlayerResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("playerGUID", params.PlayerGuid).
			Int32("mailID", params.MailID).
			Err(err).
			Msgf("Handled MarkAsReadForPlayer for %v.", time.Since(t))
	}(time.Now())

	res, err = g.mailServer.MarkAsReadForPlayer(ctx, params)
	return
}

func (g *mailDebugLoggerMiddleware) RemoveMailItem(ctx context.Context, params *pb.RemoveMailItemRequest) (res *pb.RemoveMailItemResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Uint64("itemGUID", params.ItemGuid).
			Int32("mailID", params.MailID).
			Err(err).
			Msgf("Handled RemoveMailItem for %v.", time.Since(t))
	}(time.Now())

	res, err = g.mailServer.RemoveMailItem(ctx, params)
	return
}

func (g *mailDebugLoggerMiddleware) MailByID(ctx context.Context, params *pb.MailByIDRequest) (res *pb.MailByIDResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Int32("mailID", params.MailID).
			Err(err).
			Msgf("Handled MailByID for %v.", time.Since(t))
	}(time.Now())

	res, err = g.mailServer.MailByID(ctx, params)
	return
}

func (g *mailDebugLoggerMiddleware) RemoveMailMoney(ctx context.Context, params *pb.RemoveMailMoneyRequest) (res *pb.RemoveMailMoneyResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Int32("mailID", params.MailID).
			Err(err).
			Msgf("Handled RemoveMailMoney for %v.", time.Since(t))
	}(time.Now())

	res, err = g.mailServer.RemoveMailMoney(ctx, params)
	return
}

func (g *mailDebugLoggerMiddleware) DeleteMail(ctx context.Context, params *pb.DeleteMailRequest) (res *pb.DeleteMailResponse, err error) {
	defer func(t time.Time) {
		g.logger.Debug().
			Int32("mailID", params.MailID).
			Err(err).
			Msgf("Handled DeleteMail for %v.", time.Since(t))
	}(time.Now())

	res, err = g.mailServer.DeleteMail(ctx, params)
	return
}
