package service

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

type Cleaner interface {
	ProcessExpiredMails(ctx context.Context, realmID uint32) error
}

type MailsCleanupTicker struct {
	realms  []uint32
	cleaner Cleaner
	delay   time.Duration
}

func NewMailsCleanupTicker(realms []uint32, delay time.Duration, cleaner Cleaner) *MailsCleanupTicker {
	return &MailsCleanupTicker{
		realms:  realms,
		cleaner: cleaner,
		delay:   delay,
	}
}

func (t *MailsCleanupTicker) Start(ctx context.Context) {
	ticker := time.NewTicker(t.delay)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for _, realm := range t.realms {
				if err := t.cleaner.ProcessExpiredMails(ctx, realm); err != nil {
					log.Error().Err(err).Uint32("realmID", realm).Msg("Failed to process expired mails")
				}
			}

		case <-ctx.Done():
			return
		}
	}
}
