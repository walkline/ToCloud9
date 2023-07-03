package service

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/mailserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

type MailService struct {
	repo repo.MailRepo
	ev   events.MailServiceProducer
}

func NewMailService(r repo.MailRepo, ev events.MailServiceProducer) *MailService {
	return &MailService{repo: r, ev: ev}
}

func (s *MailService) SendMail(ctx context.Context, realmID uint32, mail *repo.Mail) error {
	err := s.repo.AddMail(ctx, realmID, mail)
	if err != nil {
		return err
	}

	err = s.ev.IncomingMail(&events.MailEventIncomingMailPayload{
		RealmID:           realmID,
		SenderGUID:        mail.SenderGuid,
		ReceiverGUID:      mail.ReceiverGuid,
		DeliveryTimestamp: mail.DeliveryTimestamp,
		MailID:            uint32(mail.ID),
	})
	if err != nil {
		log.Warn().Err(err).Msg("can't send incoming mail")
	}

	return nil
}

func (s *MailService) MailListForPlayer(ctx context.Context, realmID uint32, playerGUID uint64) ([]repo.Mail, error) {
	return s.repo.MailListForPlayer(ctx, realmID, playerGUID)
}

func (s *MailService) MarkMailAsReadForPlayer(ctx context.Context, realmID uint32, playerGUID uint64, mailID uint) error {
	return s.repo.UpdateMailFlagsMaskForPlayer(ctx, realmID, playerGUID, mailID, repo.MailFlagRead)
}

func (s *MailService) MailByID(ctx context.Context, realmID uint32, mailID uint) (*repo.Mail, error) {
	return s.repo.MailByID(ctx, realmID, mailID)
}

func (s *MailService) RemoveMailItem(ctx context.Context, realmID uint32, mailID uint, itemGUID uint64, receiverGUID *uint64) error {
	var err error

	if receiverGUID != nil {
		err = s.repo.RemoveMailItemForPlayer(ctx, realmID, mailID, itemGUID, *receiverGUID)
	} else {
		err = s.repo.RemoveMailItem(ctx, realmID, mailID, itemGUID)
	}

	return err
}

func (s *MailService) RemoveMailMoney(ctx context.Context, realmID uint32, mailID uint, receiverGUID *uint64) (int32, error) {
	if receiverGUID != nil {
		return s.repo.RemoveMailMoneyForPlayer(ctx, realmID, mailID, *receiverGUID)
	}
	return s.repo.RemoveMailMoney(ctx, realmID, mailID)
}
