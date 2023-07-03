package session

import (
	"context"
	"fmt"
	"time"

	root "github.com/walkline/ToCloud9/apps/game-load-balancer"
	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbMail "github.com/walkline/ToCloud9/gen/mail/pb"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/guid"
)

func (s *GameSession) HandleSendMail(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	/*mailBox :=*/ reader.Uint64()
	target := reader.String()
	subject := reader.String()
	body := reader.String()
	/*stationaryID :=*/ reader.Int32()
	/*packageID :=*/ reader.Int32()

	attachmentsCount := reader.Uint8()
	attachmentGUIDs := make([]guid.ObjectGuid, attachmentsCount)

	for i := uint8(0); i < attachmentsCount; i++ {
		reader.Uint8()
		attachmentGUIDs[i] = guid.NewObjectGuidFromUint64(reader.Uint64())
	}

	moneyToSend := reader.Int32()
	cashOnDelivery := reader.Int32()

	playerOnline, err := s.charServiceClient.CharacterByName(ctx, &pbChar.CharacterByNameRequest{
		Api:           root.Ver,
		RealmID:       root.RealmID,
		CharacterName: target,
	})
	if err != nil {
		return err
	}

	if playerOnline.Character == nil {
		s.SendSysMessage("Player not found")
		return nil
	}

	gameClient, err := s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(s.worldSocket.Address())
	if err != nil {
		return fmt.Errorf("can't get gameClient, err: %w", err)
	}

	attachmentsToSend := make([]*pbMail.ItemAttachment, attachmentsCount)

	rawGuids := make([]uint64, len(attachmentGUIDs))
	for i, d := range attachmentGUIDs {
		rawGuids[i] = d.GetRawValue()
	}

	money, err := gameClient.GetMoneyForPlayer(ctx, &pb.GetMoneyForPlayerRequest{
		Api:        "",
		PlayerGuid: s.character.GUID,
	})
	if err != nil {
		return fmt.Errorf("can't get money, err: %w", err)
	}

	const costForOneMail = 30
	var requiredMoney int32 = costForOneMail
	if attachmentsCount > 0 {
		requiredMoney = int32(costForOneMail * attachmentsCount)
	}

	requiredMoney += moneyToSend
	isOverflow := requiredMoney < moneyToSend
	if isOverflow || uint32(requiredMoney) > money.Money {
		wr := packet.NewWriterWithSize(packet.SMsgSendMailResult, 12)
		wr.Uint32(0)
		wr.Uint32(0)
		wr.Uint32(3)
		s.gameSocket.Send(wr)
		return nil
	}

	if attachmentsCount > 0 {
		items, err := gameClient.GetPlayerItemsByGuids(ctx, &pb.GetPlayerItemsByGuidsRequest{
			Api:        "",
			PlayerGuid: s.character.GUID,
			Guids:      rawGuids,
		})
		if err != nil {
			return fmt.Errorf("can't get items for emails, err: %w", err)
		}

		// Do we need to check every GUID?
		if len(items.Items) != len(attachmentGUIDs) {
			return fmt.Errorf("attached item is absent in client, have - %d, expected - %d", len(items.Items), len(attachmentGUIDs))
		}

		for i, item := range items.Items {
			// TODO: send correct response.
			if !item.IsTradable {
				return fmt.Errorf("item is not tradable")
			}

			attachmentsToSend[i] = &pbMail.ItemAttachment{
				Guid:             uint64(guid.NewObjectGuidFromUint64(item.Guid).GetCounter()),
				Entry:            item.Entry,
				Count:            item.Count,
				Flags:            item.Flags,
				Durability:       int32(item.Durability),
				Charges:          0,
				RandomPropertyID: item.RandomPropertyID,
				PropertySeed:     0,
				Text:             item.Text,
			}
		}

		removedGUIDs, err := gameClient.RemoveItemsWithGuidsFromPlayer(ctx, &pb.RemoveItemsWithGuidsFromPlayerRequest{
			Api:                "",
			PlayerGuid:         s.character.GUID,
			Guids:              rawGuids,
			AssignToPlayerGuid: 0,
		})
		if err != nil {
			return fmt.Errorf("can't remove items from player, err: %w", err)
		}

		if len(removedGUIDs.UpdatedItemsGuids) != len(attachmentGUIDs) {
			return fmt.Errorf("removed not all items, have - %d, expected - %d", len(items.Items), len(attachmentGUIDs))
		}
	}

	_, err = gameClient.ModifyMoneyForPlayer(ctx, &pb.ModifyMoneyForPlayerRequest{
		Api:        "",
		PlayerGuid: s.character.GUID,
		Value:      -requiredMoney,
	})
	if err != nil {
		return fmt.Errorf("can't modify money, err: %w", err)
	}

	_, err = s.mailServiceClient.Send(ctx, &pbMail.SendRequest{
		Api:                 root.SupportedMailServiceVer,
		RealmID:             root.RealmID,
		SenderGuid:          &s.character.GUID,
		ReceiverGuid:        playerOnline.Character.CharGUID,
		Subject:             subject,
		Body:                body,
		MoneyToSend:         moneyToSend,
		CashOnDelivery:      cashOnDelivery,
		Attachments:         attachmentsToSend,
		DeliveryTimestamp:   time.Now().Add(time.Second * 30).Unix(),
		ExpirationTimestamp: time.Now().Add(time.Hour).Unix(),
		Type:                0,
		TemplateID:          0,
	})

	if err != nil {
		return err
	}

	wr := packet.NewWriterWithSize(packet.SMsgSendMailResult, 0)
	wr.Uint32(0)
	wr.Uint32(0)
	wr.Uint32(0)

	s.gameSocket.Send(wr)

	return nil
}

func (s *GameSession) HandleGetMailList(ctx context.Context, p *packet.Packet) error {
	resp, err := s.mailServiceClient.MailsForPlayer(ctx, &pbMail.MailsForPlayerRequest{
		Api:        root.SupportedMailServiceVer,
		RealmID:    root.RealmID,
		PlayerGuid: s.character.GUID,
	})
	if err != nil {
		return fmt.Errorf("can't fetch mail list, err: %w", err)
	}

	wr := packet.NewWriterWithSize(packet.SMsgMailListResult, 0)
	wr.Uint32(uint32(len(resp.Mails)))
	wr.Uint8(uint8(len(resp.Mails)))

	const mailStationeryDefault = 41

	for _, mail := range resp.Mails {
		var senderSize uint16 = 8
		if mail.Type != pbMail.MailType_PlayerToPlayer {
			senderSize = 4
		}

		wr.Uint16(2 + 4 + 1 + senderSize + 4 + 4 + 4 + 4 + 4 + 4 + 4 + uint16(len(mail.Subject)) + 1 + uint16(len(mail.Body)) + 1 + 1 + uint16(len(mail.Attachments))*(32+2+7*16))
		wr.Uint32(mail.Id)
		wr.Uint8(uint8(mail.Type))
		if mail.Type == pbMail.MailType_PlayerToPlayer {
			wr.Uint64(mail.Sender)
		} else {
			wr.Int32(int32(mail.Sender))
		}

		wr.Uint32(uint32(mail.CashOnDelivery))
		wr.Int32(0) //packageID?
		wr.Int32(mailStationeryDefault)
		wr.Uint32(uint32(mail.MoneyToSend))
		wr.Int32(mail.Flags)
		const daySeconds float32 = 60 * 60 * 24
		wr.Float32(float32(mail.ExpirationTimestamp-time.Now().Unix()) / daySeconds)
		wr.Int32(int32(mail.TemplateID))
		wr.String(mail.Subject)
		wr.String(mail.Body)
		wr.Uint8(uint8(len(mail.Attachments)))

		for i, attachment := range mail.Attachments {
			wr.Uint8(uint8(i + 1))
			wr.Int32(int32(attachment.Guid))
			wr.Int32(int32(attachment.Entry))
			// add enchantments support
			for i := 0; i < 7; i++ {
				wr.Int32(0)
				wr.Int32(0)
				wr.Int32(0)
			}
			wr.Int32(int32(attachment.RandomPropertyID))
			wr.Int32(int32(attachment.RandomPropertyID))
			wr.Int32(int32(attachment.Count))
			wr.Int32(attachment.Charges)
			wr.Int32(attachment.Durability) // MaxDurability
			wr.Int32(attachment.Durability)
			wr.Uint8(0)
		}
	}

	s.gameSocket.Send(wr)

	return nil
}

func (s *GameSession) HandleMailMarksAsRead(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	/*mailBox*/ _ = reader.Uint64() // TODO: Add interaction check.
	mailID := reader.Int32()

	_, err := s.mailServiceClient.MarkAsReadForPlayer(ctx, &pbMail.MarkAsReadForPlayerRequest{
		Api:        root.SupportedMailServiceVer,
		RealmID:    root.RealmID,
		PlayerGuid: s.character.GUID,
		MailID:     mailID,
	})
	if err != nil {
		return fmt.Errorf("can't mark mail as read, err: %w", err)
	}

	return nil
}

func (s *GameSession) HandleMailTakeMoney(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	/*mailBox :=*/ reader.Uint64()
	mailID := reader.Int32()

	gameClient, err := s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(s.worldSocket.Address())
	if err != nil {
		return fmt.Errorf("can't get gameClient, err: %w", err)
	}

	removeResult, err := s.mailServiceClient.RemoveMailMoney(ctx, &pbMail.RemoveMailMoneyRequest{})
	if err != nil {
		return err
	}

	if removeResult.MoneyRemoved == 0 {
		wr := packet.NewWriterWithSize(packet.SMsgSendMailResult, 12)
		wr.Int32(mailID)
		wr.Uint32(1)
		wr.Uint32(6)
		s.gameSocket.Send(wr)
		return nil
	}

	_, err = gameClient.ModifyMoneyForPlayer(ctx, &pb.ModifyMoneyForPlayerRequest{
		Api:        "",
		PlayerGuid: s.character.GUID,
		Value:      int32(removeResult.MoneyRemoved),
	})
	if err != nil {
		return err
	}

	wr := packet.NewWriterWithSize(packet.SMsgSendMailResult, 12)
	wr.Int32(mailID)
	wr.Uint32(1)
	wr.Uint32(0)
	s.gameSocket.Send(wr)

	return nil
}

func (s *GameSession) HandleMailTakeItem(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	/*mailBox*/ _ = reader.Uint64() // TODO: Add interaction check.
	mailID := reader.Int32()
	itemID := reader.Int32()

	mailResp, err := s.mailServiceClient.MailByID(ctx, &pbMail.MailByIDRequest{
		Api:     root.SupportedMailServiceVer,
		RealmID: root.RealmID,
		MailID:  mailID,
	})
	if err != nil {
		return err
	}

	var item *pbMail.ItemAttachment
	for _, attachment := range mailResp.Mail.Attachments {
		if attachment.Guid == uint64(itemID) {
			item = attachment
			break
		}
	}

	if item == nil || mailResp.Mail.ReceiverGuid != s.character.GUID {
		return fmt.Errorf("item %d not found in the given mail %d", itemID, mailID)
	}

	gameClient, err := s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(s.worldSocket.Address())
	if err != nil {
		return fmt.Errorf("can't get gameClient, err: %w", err)
	}

	addResp, err := gameClient.AddExistingItemToPlayer(ctx, &pb.AddExistingItemToPlayerRequest{
		Api:        "",
		PlayerGuid: s.character.GUID,
		Item: &pb.AddExistingItemToPlayerRequest_Item{
			Guid:             guid.NewObjectGuidFromValues(guid.Item, item.Entry, guid.LowType(itemID)).GetRawValue(),
			Entry:            item.Entry,
			Count:            item.Count,
			Flags:            item.Flags,
			Durability:       uint32(item.Durability),
			RandomPropertyID: item.RandomPropertyID,
			Text:             item.Text,
		},
	})
	if err != nil {
		return err
	}

	if addResp.Status == pb.AddExistingItemToPlayerResponse_NoSpace {
		wr := packet.NewWriterWithSize(packet.SMsgSendMailResult, 0)
		wr.Uint32(uint32(mailID))
		wr.Uint32(2)
		wr.Uint32(1)
		wr.Uint32(4)
		s.gameSocket.Send(wr)
		return nil
	}

	_, err = s.mailServiceClient.RemoveMailItem(ctx, &pbMail.RemoveMailItemRequest{
		Api:        root.SupportedMailServiceVer,
		RealmID:    root.RealmID,
		PlayerGuid: &s.character.GUID,
		MailID:     mailID,
		ItemGuid:   uint64(itemID),
	})
	if err != nil {
		return err
	}

	wr := packet.NewWriterWithSize(packet.SMsgSendMailResult, 0)
	wr.Uint32(uint32(mailID))
	wr.Uint32(2)
	wr.Uint32(0)
	wr.Int32(itemID)
	wr.Uint32(item.Count)
	s.gameSocket.Send(wr)

	return nil
}

func (s *GameSession) HandleQueryNextMailTime(ctx context.Context, p *packet.Packet) error {
	resp, err := s.mailServiceClient.MailsForPlayer(ctx, &pbMail.MailsForPlayerRequest{
		Api:        root.SupportedMailServiceVer,
		RealmID:    root.RealmID,
		PlayerGuid: s.character.GUID,
	})
	if err != nil {
		return fmt.Errorf("can't fetch mail list, err: %w", err)
	}

	const MailReadFlag = 1
	unreadMails := []*pbMail.Mail{}
	for i, mail := range resp.Mails {
		if mail.Flags&MailReadFlag != 0 {
			continue
		}

		unreadMails = append(unreadMails, resp.Mails[i])

		if len(unreadMails) > 2 {
			break
		}
	}

	if len(unreadMails) == 0 {
		wr := packet.NewWriterWithSize(packet.MsgQueryNextMailTime, 8)
		wr.Float32(-float32((time.Hour * 24).Seconds()))
		wr.Uint32(0)
		s.gameSocket.Send(wr)
		return nil
	}

	nextCheck := unreadMails[len(unreadMails)-1].DeliveryTimestamp - time.Now().Unix()
	if nextCheck < 0 {
		nextCheck = 0
	}
	wr := packet.NewWriterWithSize(packet.MsgQueryNextMailTime, 0)
	wr.Float32(float32(nextCheck))
	wr.Uint32(uint32(len(unreadMails)))

	for _, mail := range unreadMails {
		wr.Uint64(mail.Sender)
		wr.Uint32(0)
		wr.Uint32(uint32(mail.Type))
		wr.Uint32(41)
		wr.Float32(float32(mail.DeliveryTimestamp - time.Now().Unix()))
	}

	s.gameSocket.Send(wr)

	return nil
}

func (s *GameSession) HandleEventIncomingMail(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.MailEventIncomingMailPayload)

	delay := float32(eventData.DeliveryTimestamp - time.Now().Unix())
	if delay < 0 {
		delay = 0
	}

	wr := packet.NewWriterWithSize(packet.SMsgReceivedMail, 4)
	wr.Float32(delay)
	s.gameSocket.Send(wr)

	return nil
}
