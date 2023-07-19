package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/mailserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

var (
	ErrNotEnoughRight = errors.New("not enough rights")
	ErrNoReceiver     = errors.New("receiver can't be null")
)

type MailService struct {
	repo repo.MailRepo
	ev   events.MailServiceProducer

	defaultMailExpirationTime time.Duration
}

func NewMailService(r repo.MailRepo, ev events.MailServiceProducer, defaultMailExpirationTime time.Duration) *MailService {
	return &MailService{repo: r, ev: ev, defaultMailExpirationTime: defaultMailExpirationTime}
}

func (s *MailService) SendMail(ctx context.Context, realmID uint32, mail *repo.Mail) error {
	if mail.ReceiverGuid == 0 {
		return ErrNoReceiver
	}
	if mail.ExpirationTimestamp == 0 {
		mail.ExpirationTimestamp = time.Now().Add(s.defaultMailExpirationTime).Unix()
	}

	if mail.DeliveryTimestamp == 0 {
		// Add some buffer.
		// May fix issue on client side when you need to wait for mail list refresh.
		mail.DeliveryTimestamp = time.Now().Add(time.Second * 5).Unix()
	}

	if mail.Stationery == 0 {
		mail.Stationery = uint8(repo.MailStationeryTypeDefault)
	}

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
	mail, err := s.repo.MailByID(ctx, realmID, mailID)
	if err != nil {
		return err
	}
	return s.repo.UpdateMailFlagsMaskForPlayer(ctx, realmID, playerGUID, mailID, repo.MailFlagMask(mail.FlagsMask)|repo.MailFlagRead)
}

func (s *MailService) MailByID(ctx context.Context, realmID uint32, mailID uint) (*repo.Mail, error) {
	return s.repo.MailByID(ctx, realmID, mailID)
}

func (s *MailService) RemoveMailItem(ctx context.Context, realmID uint32, mailID uint, itemGUID uint64, receiverGUID *uint64, handleCOD bool) error {
	mail, err := s.repo.MailByID(ctx, realmID, mailID)
	if err != nil {
		return err
	}

	if receiverGUID != nil && mail.ReceiverGuid != *receiverGUID {
		return ErrNotEnoughRight
	}

	err = s.repo.RemoveMailItem(ctx, realmID, mailID, itemGUID)
	if err != nil {
		return err
	}

	needsUpdate := false

	if mail.HasItemAttachments && len(mail.Attachments) == 1 && mail.Attachments[0].GUID == itemGUID {
		mail.HasItemAttachments = false
		needsUpdate = true
	}

	if handleCOD && mail.CashOnDelivery > 0 {
		err = s.SendMail(ctx, realmID, &repo.Mail{
			SenderGuid:   mail.ReceiverGuid,
			ReceiverGuid: mail.SenderGuid,
			FlagsMask:    uint16(repo.MailFlagCashOnDelivery),
			Subject:      mail.Subject,
			MoneyToSend:  mail.CashOnDelivery,
		})
		if err != nil {
			return fmt.Errorf("failed to send cash on delivery mail, err: %w", err)
		}

		mail.CashOnDelivery = 0
		needsUpdate = true
	}

	if needsUpdate {
		if err = s.repo.UpdateMailWithoutAttachments(ctx, realmID, mail); err != nil {
			return err
		}
	}

	return err
}

func (s *MailService) RemoveMailMoney(ctx context.Context, realmID uint32, mailID uint, receiverGUID *uint64) (int32, error) {
	if receiverGUID != nil {
		return s.repo.RemoveMailMoneyForPlayer(ctx, realmID, mailID, *receiverGUID)
	}
	return s.repo.RemoveMailMoney(ctx, realmID, mailID)
}

func (s *MailService) DeleteMail(ctx context.Context, realmID uint32, mailID uint) error {
	mail, err := s.repo.MailByID(ctx, realmID, mailID)
	if err != nil {
		return err
	}

	attachmentIDs := make([]uint64, len(mail.Attachments))
	for i, attachment := range mail.Attachments {
		attachmentIDs[i] = attachment.GUID
	}

	if len(attachmentIDs) > 0 {
		err = s.repo.DeleteMailItemsWithIDs(ctx, realmID, attachmentIDs)
		if err != nil {
			return err
		}

		err = s.repo.DeleteItemsWithIDs(ctx, realmID, attachmentIDs)
		if err != nil {
			return err
		}
	}

	return s.repo.DeleteMailsWithoutAttachments(ctx, realmID, []uint{mailID})
}

func (s *MailService) ProcessExpiredMails(ctx context.Context, realmID uint32) error {
	mails, err := s.repo.ExpiredMails(ctx, realmID)
	if err != nil {
		return err
	}

	mailsIDsWithoutAttachments := []uint{}
	mailsWithAttachments := []repo.Mail{}
	for i := range mails {
		if mails[i].MoneyToSend != 0 || mails[i].HasItemAttachments {
			mailsWithAttachments = append(mailsWithAttachments, mails[i])
		} else {
			mailsIDsWithoutAttachments = append(mailsIDsWithoutAttachments, mails[i].ID)
		}
	}

	if len(mailsIDsWithoutAttachments) != 0 {
		err = s.repo.DeleteMailsWithoutAttachments(ctx, realmID, mailsIDsWithoutAttachments)
		if err != nil {
			return err
		}
	}

	mailsIDsToDelete := []uint{}
	for _, mail := range mailsWithAttachments {
		if mail.FlagsMask&uint16(repo.MailFlagReturned) > 0 {
			mailsIDsToDelete = append(mailsIDsToDelete, mail.ID)
			continue
		}

		mail.FlagsMask |= uint16(repo.MailFlagReturned)
		mail.ReceiverGuid = mail.SenderGuid
		mail.ExpirationTimestamp = time.Now().Add(time.Second * time.Duration(mail.ExpirationTimestamp-mail.DeliveryTimestamp)).Unix()
		err = s.repo.UpdateMailWithoutAttachments(ctx, realmID, &mail)
		if err != nil {
			return err
		}

		if mail.HasItemAttachments {
			err = s.repo.UpdateMailItemsOwner(ctx, realmID, mail.ID, mail.SenderGuid)
			if err != nil {
				return err
			}
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

	}

	if len(mailsIDsToDelete) == 0 {
		return nil
	}

	mailItemsToDelete, err := s.repo.MailItemsIDsByMailIDs(ctx, realmID, mailsIDsToDelete)
	if err != nil {
		return err
	}

	if len(mailItemsToDelete) > 0 {
		err = s.repo.DeleteItemsWithIDs(ctx, realmID, mailItemsToDelete)
		if err != nil {
			return err
		}

		err = s.repo.DeleteMailItemsWithIDs(ctx, realmID, mailItemsToDelete)
		if err != nil {
			return err
		}
	}

	err = s.repo.DeleteMailsWithoutAttachments(ctx, realmID, mailsIDsToDelete)
	if err != nil {
		return err
	}

	return nil
}
