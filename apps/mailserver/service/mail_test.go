package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/walkline/ToCloud9/apps/mailserver/repo"
	"github.com/walkline/ToCloud9/apps/mailserver/repo/mocks"
	eventsMock "github.com/walkline/ToCloud9/shared/events/mocks"
)

func TestMailService_SendMail(t *testing.T) {
	expirationTime := time.Second
	tests := map[string]struct {
		mail    *repo.Mail
		expMail *repo.Mail
		wantErr bool
	}{
		"mail with all fields": {
			mail: &repo.Mail{
				Type:                repo.MailTypePlayerToPlayer,
				Stationery:          uint8(repo.MailStationeryTypeGM),
				TemplateID:          42,
				SenderGuid:          22,
				ReceiverGuid:        21,
				FlagsMask:           3,
				Subject:             "Hi there",
				Body:                "Body message",
				MoneyToSend:         55,
				CashOnDelivery:      21,
				DeliveryTimestamp:   time.Now().Unix(),
				ExpirationTimestamp: time.Now().Add(expirationTime).Unix(),
				HasItemAttachments:  true,
				Attachments: []repo.ItemAttachment{
					{
						GUID:             1,
						OwnerGUID:        21,
						Entry:            4,
						Pos:              5,
						Count:            2,
						Charges:          21,
						Flags:            31,
						RandomPropertyID: 67,
						PropertySeed:     41,
						Durability:       78,
						Text:             "123",
					},
				},
			},
			expMail: &repo.Mail{
				Type:                repo.MailTypePlayerToPlayer,
				Stationery:          uint8(repo.MailStationeryTypeGM),
				TemplateID:          42,
				SenderGuid:          22,
				ReceiverGuid:        21,
				FlagsMask:           3,
				Subject:             "Hi there",
				Body:                "Body message",
				MoneyToSend:         55,
				CashOnDelivery:      21,
				DeliveryTimestamp:   time.Now().Unix(),
				ExpirationTimestamp: time.Now().Add(expirationTime).Unix(),
				HasItemAttachments:  true,
				Attachments: []repo.ItemAttachment{
					{
						GUID:             1,
						OwnerGUID:        21,
						Entry:            4,
						Pos:              5,
						Count:            2,
						Charges:          21,
						Flags:            31,
						RandomPropertyID: 67,
						PropertySeed:     41,
						Durability:       78,
						Text:             "123",
					},
				},
			},
		},
		"mail without optional fields": {
			mail: &repo.Mail{
				Type:         repo.MailTypePlayerToPlayer,
				ReceiverGuid: 1,
				Subject:      "Hi there",
				Body:         "Body message",
			},
			expMail: &repo.Mail{
				Type:                repo.MailTypePlayerToPlayer,
				ReceiverGuid:        1,
				Subject:             "Hi there",
				Body:                "Body message",
				Stationery:          uint8(repo.MailStationeryTypeDefault),
				DeliveryTimestamp:   time.Now().Add(time.Second * 5).Unix(),
				ExpirationTimestamp: time.Now().Add(expirationTime).Unix(),
			},
			wantErr: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mailRepo := &mocks.MailRepo{}
			mailRepo.On("AddMail", mock.Anything, mock.Anything, mock.MatchedBy(func(m *repo.Mail) bool {
				assert.Equal(t, *tt.expMail, *m)
				return true
			})).Return(nil)

			eventsProducer := &eventsMock.MailServiceProducer{}
			eventsProducer.On("IncomingMail", mock.Anything).Return(nil)

			s := &MailService{
				repo:                      mailRepo,
				ev:                        eventsProducer,
				defaultMailExpirationTime: expirationTime,
			}

			if err := s.SendMail(context.Background(), 1, tt.mail); (err != nil) != tt.wantErr {
				t.Errorf("SendMail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMailService_RemoveMailItemWithCOD(t *testing.T) {
	cod := int32(4422)
	mail := &repo.Mail{
		ID:                  42,
		Type:                repo.MailTypePlayerToPlayer,
		SenderGuid:          2,
		ReceiverGuid:        1,
		Subject:             "Hi there",
		Body:                "Body message",
		CashOnDelivery:      cod,
		Stationery:          uint8(repo.MailStationeryTypeDefault),
		DeliveryTimestamp:   time.Now().Add(time.Second * 5).Unix(),
		ExpirationTimestamp: time.Now().Add(time.Second * 50).Unix(),
		HasItemAttachments:  true,
		Attachments: []repo.ItemAttachment{
			{
				GUID:      1,
				OwnerGUID: 21,
				Entry:     4,
				Text:      "123",
			},
		},
	}

	mailRepo := &mocks.MailRepo{}

	mailRepo.On("MailByID", mock.Anything, mock.Anything, mail.ID).Return(mail, nil)
	mailRepo.On("RemoveMailItem", mock.Anything, mock.Anything, mail.ID, mail.Attachments[0].GUID).Return(nil)
	mailRepo.On("UpdateMailWithoutAttachments", mock.Anything, mock.Anything, mock.MatchedBy(func(m *repo.Mail) bool {
		assert.Equal(t, int32(0), m.CashOnDelivery)
		assert.Equal(t, false, m.HasItemAttachments)
		return true
	})).Return(nil)
	mailRepo.On("AddMail", mock.Anything, mock.Anything, mock.MatchedBy(func(m *repo.Mail) bool {
		assert.Equal(t, mail.ReceiverGuid, m.SenderGuid)
		assert.Equal(t, mail.SenderGuid, m.ReceiverGuid)
		assert.Equal(t, int32(0), m.CashOnDelivery)
		assert.Equal(t, cod, m.MoneyToSend)
		assert.Equal(t, uint16(repo.MailFlagCashOnDelivery), m.FlagsMask)
		assert.Equal(t, mail.Subject, m.Subject)
		return true
	})).Return(nil)

	eventsProducer := &eventsMock.MailServiceProducer{}
	eventsProducer.On("IncomingMail", mock.Anything).Return(nil)

	s := &MailService{
		repo: mailRepo,
		ev:   eventsProducer,
	}

	err := s.RemoveMailItem(context.Background(), 1, mail.ID, mail.Attachments[0].GUID, &mail.ReceiverGuid, true)
	assert.NoError(t, err)
}
