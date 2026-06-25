package service

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/shared/events"
)

const (
	lfgMaterializerQueueGroup = "matchmaking_lfg_materializer"
	// Accepted proposal materialization runs after the client answer window and
	// includes crossrealm redirects plus target-world loading.
	lfgMaterializerAcceptedProposalTimeout = 90 * time.Second
)

type LFGMaterializerListener struct {
	nc           *nats.Conn
	materializer *LFGMaterializer
	lfgService   LFGService
	subs         []*nats.Subscription
}

func NewLFGMaterializerListener(nc *nats.Conn, materializer *LFGMaterializer, lfgService LFGService) *LFGMaterializerListener {
	return &LFGMaterializerListener{
		nc:           nc,
		materializer: materializer,
		lfgService:   lfgService,
	}
}

func (l *LFGMaterializerListener) Listen() error {
	if l.nc == nil {
		return errors.New("lfg materializer listener nats connection is nil")
	}
	if l.materializer == nil {
		return errors.New("lfg materializer listener materializer is nil")
	}
	if l.lfgService == nil {
		return errors.New("lfg materializer listener lfg service is nil")
	}

	sub, err := l.nc.QueueSubscribe(events.MatchmakingEventLfgProposalAccepted.SubjectName(), lfgMaterializerQueueGroup, func(msg *nats.Msg) {
		payload := events.MatchmakingEventLfgProposalAcceptedPayload{}
		if _, err := events.Unmarshal(msg.Data, &payload); err != nil {
			log.Error().Err(err).Msg("can't read MatchmakingEventLfgProposalAccepted event")
			return
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), lfgMaterializerAcceptedProposalTimeout)
			defer cancel()

			if err := l.materializer.MaterializeAcceptedProposal(ctx, &payload); err != nil {
				log.Error().
					Err(err).
					Uint32("realmID", payload.RealmID).
					Uint32("proposalID", payload.ProposalID).
					Str("leaderWorldserverID", payload.LeaderWorldserverID).
					Msg("failed to materialize LFG proposal")
				if failErr := l.lfgService.FailLfgProposal(ctx, payload.RealmID, payload.ProposalID); failErr != nil {
					log.Error().
						Err(failErr).
						Uint32("realmID", payload.RealmID).
						Uint32("proposalID", payload.ProposalID).
						Msg("failed to mark LFG proposal as failed after materialization error")
				}
			}
		}()
	})
	if err != nil {
		l.unsubscribe()
		return err
	}

	l.subs = append(l.subs, sub)
	return nil
}

func (l *LFGMaterializerListener) Stop() error {
	return l.unsubscribe()
}

func (l *LFGMaterializerListener) unsubscribe() error {
	for _, sub := range l.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}
	return nil
}
