package server

import (
	"context"

	"github.com/walkline/ToCloud9/apps/mailserver"
	"github.com/walkline/ToCloud9/apps/mailserver/repo"
	"github.com/walkline/ToCloud9/apps/mailserver/service"
	"github.com/walkline/ToCloud9/gen/mail/pb"
)

type MailServer struct {
	service *service.MailService
}

func NewMailServer(mailService *service.MailService) pb.MailServiceServer {
	return &MailServer{service: mailService}
}

func (m *MailServer) Send(ctx context.Context, request *pb.SendRequest) (*pb.SendResponse, error) {
	var senderGuid uint64 = 0
	if request.SenderGuid != nil {
		senderGuid = *request.SenderGuid
	}

	attachments := make([]repo.ItemAttachment, len(request.Attachments))
	for i, attachment := range request.Attachments {
		attachments[i].GUID = attachment.Guid
		attachments[i].Entry = uint(attachment.Entry)
		attachments[i].Flags = uint(attachment.Flags)
		attachments[i].Count = int(attachment.Count)
		attachments[i].RandomPropertyID = uint(attachment.RandomPropertyID)
		attachments[i].Durability = int(attachment.Durability)
		attachments[i].Charges = int(attachment.Durability)
		attachments[i].Text = attachment.Text
	}

	mail := repo.Mail{
		Type:                repo.MailType(request.Type),
		Stationery:          uint8(request.Stationery),
		TemplateID:          request.TemplateID,
		SenderGuid:          senderGuid,
		ReceiverGuid:        request.ReceiverGuid,
		Subject:             request.Subject,
		Body:                request.Body,
		MoneyToSend:         request.MoneyToSend,
		CashOnDelivery:      request.CashOnDelivery,
		DeliveryTimestamp:   request.DeliveryTimestamp,
		ExpirationTimestamp: request.ExpirationTimestamp,
		FlagsMask:           uint16(request.FlagsMask),
		Attachments:         attachments,
	}
	err := m.service.SendMail(ctx, request.RealmID, &mail)
	if err != nil {
		return nil, err
	}

	return &pb.SendResponse{
		Api:    mailserver.Ver,
		MailID: uint32(mail.ID),
	}, nil
}

func (m *MailServer) MailsForPlayer(ctx context.Context, request *pb.MailsForPlayerRequest) (*pb.MailsForPlayerResponse, error) {
	mails, err := m.service.MailListForPlayer(ctx, request.RealmID, request.PlayerGuid)
	if err != nil {
		return nil, err
	}

	mailsToReturn := make([]*pb.Mail, len(mails))
	for i, mail := range mails {
		mailAttachments := make([]*pb.ItemAttachment, len(mail.Attachments))
		for i, attachment := range mail.Attachments {
			mailAttachments[i] = &pb.ItemAttachment{
				Guid:             attachment.GUID,
				Entry:            uint32(attachment.Entry),
				Count:            uint32(attachment.Count),
				Flags:            uint32(attachment.Flags),
				Durability:       int32(attachment.Durability),
				Charges:          int32(attachment.Charges),
				RandomPropertyID: uint32(attachment.RandomPropertyID),
				PropertySeed:     uint32(attachment.PropertySeed),
				Text:             attachment.Text,
			}
		}

		mailsToReturn[i] = &pb.Mail{
			Id:                  uint32(mail.ID),
			Sender:              mail.SenderGuid,
			ReceiverGuid:        mail.ReceiverGuid,
			Subject:             mail.Subject,
			Body:                mail.Body,
			MoneyToSend:         mail.MoneyToSend,
			CashOnDelivery:      mail.CashOnDelivery,
			Attachments:         mailAttachments,
			DeliveryTimestamp:   mail.DeliveryTimestamp,
			ExpirationTimestamp: mail.ExpirationTimestamp,
			Flags:               int32(mail.FlagsMask),
			Type:                pb.MailType(mail.Type),
			TemplateID:          mail.TemplateID,
			Stationery:          pb.MailStationery(mail.Stationery),
		}
	}
	return &pb.MailsForPlayerResponse{
		Api:   mailserver.Ver,
		Mails: mailsToReturn,
	}, nil
}

func (m *MailServer) MarkAsReadForPlayer(ctx context.Context, request *pb.MarkAsReadForPlayerRequest) (*pb.MarkAsReadForPlayerResponse, error) {
	if err := m.service.MarkMailAsReadForPlayer(ctx, request.RealmID, request.PlayerGuid, uint(request.MailID)); err != nil {
		return nil, err
	}

	return &pb.MarkAsReadForPlayerResponse{
		Api: mailserver.Ver,
	}, nil
}

func (m *MailServer) MailByID(ctx context.Context, request *pb.MailByIDRequest) (*pb.MailByIDResponse, error) {
	mail, err := m.service.MailByID(ctx, request.RealmID, uint(request.MailID))
	if err != nil {
		return nil, err
	}

	mailAttachments := make([]*pb.ItemAttachment, len(mail.Attachments))
	for i, attachment := range mail.Attachments {
		mailAttachments[i] = &pb.ItemAttachment{
			Guid:             attachment.GUID,
			Entry:            uint32(attachment.Entry),
			Count:            uint32(attachment.Count),
			Flags:            uint32(attachment.Flags),
			Durability:       int32(attachment.Durability),
			Charges:          int32(attachment.Charges),
			RandomPropertyID: uint32(attachment.RandomPropertyID),
			PropertySeed:     uint32(attachment.PropertySeed),
			Text:             attachment.Text,
		}
	}

	return &pb.MailByIDResponse{
		Api: mailserver.Ver,
		Mail: &pb.Mail{
			Id:                  uint32(mail.ID),
			Sender:              mail.SenderGuid,
			ReceiverGuid:        mail.ReceiverGuid,
			Subject:             mail.Subject,
			Body:                mail.Body,
			MoneyToSend:         mail.MoneyToSend,
			CashOnDelivery:      mail.CashOnDelivery,
			Attachments:         mailAttachments,
			DeliveryTimestamp:   mail.DeliveryTimestamp,
			ExpirationTimestamp: mail.ExpirationTimestamp,
			Flags:               int32(mail.FlagsMask),
			Type:                pb.MailType(mail.Type),
			TemplateID:          mail.TemplateID,
			Stationery:          pb.MailStationery(mail.Stationery),
		},
	}, nil
}

func (m *MailServer) RemoveMailItem(ctx context.Context, request *pb.RemoveMailItemRequest) (*pb.RemoveMailItemResponse, error) {
	err := m.service.RemoveMailItem(
		ctx,
		request.RealmID,
		uint(request.MailID),
		request.ItemGuid,
		request.PlayerGuid,
		request.HandleCashOnDelivery,
	)
	if err != nil {
		return nil, err
	}

	return &pb.RemoveMailItemResponse{
		Api: mailserver.Ver,
	}, nil
}

func (m *MailServer) RemoveMailMoney(ctx context.Context, request *pb.RemoveMailMoneyRequest) (*pb.RemoveMailMoneyResponse, error) {
	money, err := m.service.RemoveMailMoney(ctx, request.RealmID, uint(request.MailID), request.PlayerGuid)
	if err != nil {
		return nil, err
	}

	return &pb.RemoveMailMoneyResponse{
		Api:          mailserver.Ver,
		MoneyRemoved: money,
	}, nil
}

func (m *MailServer) DeleteMail(ctx context.Context, request *pb.DeleteMailRequest) (*pb.DeleteMailResponse, error) {
	err := m.service.DeleteMail(ctx, request.RealmID, uint(request.MailID))
	if err != nil {
		return nil, err
	}

	return &pb.DeleteMailResponse{
		Api: mailserver.Ver,
	}, nil
}
