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
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

const MaxVisibleMailsCount = 100

type MailResponseType uint8

const (
	MailResponseTypeMailSend MailResponseType = iota
	MailResponseTypeMoneyTaken
	MailResponseTypeItemTaken
	MailResponseTypeReturnedToSender
	MailResponseTypeDeleted
	MailResponseTypeMadePermanent
)

type MailResponseStatus uint8

const (
	MailResponseStatusOk MailResponseStatus = iota
	MailResponseStatusEquipErr
	MailResponseStatusCannotSendToSelf
	MailResponseStatusNotEnoughMoney
	MailResponseStatusRecipientNotFound
	MailResponseStatusNotYourTeam
	MailResponseStatusInternalError
	MailResponseStatusDisabledForTrialAcc
	MailResponseStatusRecipientCapReached
	MailResponseStatusCantSendWrappedCOD
	MailResponseStatusMailAndChatSuspended
	MailResponseStatusTooManyAttachments
	MailResponseStatusMailAttachmentInvalid
	MailResponseStatusItemHasExpired
)

type mailResult struct {
	MailID         uint
	Type           MailResponseType
	Status         MailResponseStatus
	BagResult      uint32
	AttachID       uint32
	QtyInInventory uint32
}

func (r mailResult) BuildPacket() *packet.Packet {
	w := packet.NewWriterWithSize(packet.SMsgSendMailResult, 12)
	w.Uint32(uint32(r.MailID))
	w.Uint32(uint32(r.Type))
	w.Uint32(uint32(r.Status))

	if r.Status == MailResponseStatusEquipErr {
		w.Uint32(r.BagResult)
	}

	if r.Type == MailResponseTypeItemTaken {
		if r.Status == MailResponseStatusOk || r.Status == MailResponseStatusEquipErr {
			w.Uint32(r.AttachID)
			w.Uint32(r.QtyInInventory)
		}
	}

	return w.ToPacket()
}

func (s *GameSession) HandleSendMail(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	mailBox := reader.Uint64()
	target := reader.String()
	subject := reader.String()
	body := reader.String()
	/*stationaryID :=*/ reader.Int32()
	/*packageID :=*/ reader.Int32()

	canInteract, err := s.CanInteractWithMailingObject(ctx, mailBox)
	if err != nil {
		return err
	}
	if !canInteract {
		return fmt.Errorf("player '%d' tried to interact with '%d' object, that not in reach", s.character.GUID, mailBox)
	}

	attachmentsCount := reader.Uint8()
	attachmentGUIDs := make([]guid.ObjectGuid, attachmentsCount)

	for i := uint8(0); i < attachmentsCount; i++ {
		reader.Uint8()
		attachmentGUIDs[i] = guid.New(reader.Uint64())
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
		s.gameSocket.SendPacket(mailResult{
			Type:   MailResponseTypeMailSend,
			Status: MailResponseStatusRecipientNotFound,
		}.BuildPacket())
		return nil
	}

	attachmentsToSend := make([]*pbMail.ItemAttachment, attachmentsCount)

	rawGuids := make([]uint64, len(attachmentGUIDs))
	for i, d := range attachmentGUIDs {
		rawGuids[i] = d.GetRawValue()
	}

	money, err := s.gameServerGRPCClient.GetMoneyForPlayer(ctx, &pb.GetMoneyForPlayerRequest{
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
		s.gameSocket.SendPacket(mailResult{
			Type:   MailResponseTypeMailSend,
			Status: MailResponseStatusNotEnoughMoney,
		}.BuildPacket())
		return nil
	}

	if attachmentsCount > 0 {
		items, err := s.gameServerGRPCClient.GetPlayerItemsByGuids(ctx, &pb.GetPlayerItemsByGuidsRequest{
			Api:        "",
			PlayerGuid: s.character.GUID,
			Guids:      rawGuids,
		})
		if err != nil {
			s.gameSocket.SendPacket(mailResult{
				Type:   MailResponseTypeMailSend,
				Status: MailResponseStatusMailAttachmentInvalid,
			}.BuildPacket())
			return fmt.Errorf("can't get items for mails, err: %w", err)
		}

		// Do we need to check every GUID?
		if len(items.Items) != len(attachmentGUIDs) {
			s.gameSocket.SendPacket(mailResult{
				Type:   MailResponseTypeMailSend,
				Status: MailResponseStatusMailAttachmentInvalid,
			}.BuildPacket())
			return fmt.Errorf("attached item is absent in client, have - %d, expected - %d", len(items.Items), len(attachmentGUIDs))
		}

		for i, item := range items.Items {
			if !item.IsTradable {
				s.gameSocket.SendPacket(mailResult{
					Type:   MailResponseTypeMailSend,
					Status: MailResponseStatusMailAttachmentInvalid,
				}.BuildPacket())
				return nil
			}

			attachmentsToSend[i] = &pbMail.ItemAttachment{
				Guid:             uint64(guid.New(item.Guid).GetCounter()),
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

		removedGUIDs, err := s.gameServerGRPCClient.RemoveItemsWithGuidsFromPlayer(ctx, &pb.RemoveItemsWithGuidsFromPlayerRequest{
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

	_, err = s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pb.ModifyMoneyForPlayerRequest{
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
		Stationery:          pbMail.MailStationery_StDefault,
		ExpirationTimestamp: 0,
		Type:                0,
		TemplateID:          0,
	})
	if err != nil {
		return NewMailServiceUnavailableErr(err)
	}

	s.gameSocket.SendPacket(mailResult{
		Type:   MailResponseTypeMailSend,
		Status: MailResponseStatusOk,
	}.BuildPacket())

	return nil
}

func (s *GameSession) HandleGetMailList(ctx context.Context, p *packet.Packet) error {
	mailBox := p.Reader().Uint64()

	canInteract, err := s.CanInteractWithMailingObject(ctx, mailBox)
	if err != nil {
		return err
	}
	if !canInteract {
		return fmt.Errorf("player '%d' tried to interact with '%d' object, that not in reach", s.character.GUID, mailBox)
	}

	resp, err := s.mailServiceClient.MailsForPlayer(ctx, &pbMail.MailsForPlayerRequest{
		Api:        root.SupportedMailServiceVer,
		RealmID:    root.RealmID,
		PlayerGuid: s.character.GUID,
	})
	if err != nil {
		return NewMailServiceUnavailableErr(fmt.Errorf("can't fetch mail list, err: %w", err))
	}

	wr := packet.NewWriterWithSize(packet.SMsgMailListResult, 0)
	wr.Uint32(uint32(len(resp.Mails)))
	wr.Uint8(uint8(len(resp.Mails)))

	for i, mail := range resp.Mails {
		// Let's ignore the rest. Can be issues on client side.
		if i == MaxVisibleMailsCount {
			break
		}

		var senderSize uint16 = 8
		if mail.Type != pbMail.MailType_PlayerToPlayer {
			senderSize = 4
		}

		// TODO: this looks ugly
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
		wr.Int32(int32(mail.Stationery))
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
			// TODO: add enchantments support
			for i := 0; i < 7; i++ {
				wr.Int32(0)
				wr.Int32(0)
				wr.Int32(0)
			}
			wr.Int32(int32(attachment.RandomPropertyID))
			wr.Int32(int32(attachment.RandomPropertyID))
			wr.Int32(int32(attachment.Count))
			wr.Int32(attachment.Charges)
			wr.Int32(attachment.Durability) // TODO: Add MaxDurability
			wr.Int32(attachment.Durability)
			wr.Uint8(0)
		}
	}

	s.gameSocket.Send(wr)

	return nil
}

func (s *GameSession) HandleMailMarksAsRead(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	mailBox := reader.Uint64()
	mailID := reader.Int32()

	canInteract, err := s.CanInteractWithMailingObject(ctx, mailBox)
	if err != nil {
		return err
	}
	if !canInteract {
		return fmt.Errorf("player '%d' tried to interact with '%d' object, that not in reach", s.character.GUID, mailBox)
	}

	_, err = s.mailServiceClient.MarkAsReadForPlayer(ctx, &pbMail.MarkAsReadForPlayerRequest{
		Api:        root.SupportedMailServiceVer,
		RealmID:    root.RealmID,
		PlayerGuid: s.character.GUID,
		MailID:     mailID,
	})
	if err != nil {
		return NewMailServiceUnavailableErr(fmt.Errorf("can't mark mail as read, err: %w", err))
	}

	return nil
}

func (s *GameSession) HandleMailTakeMoney(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	mailBox := reader.Uint64()
	mailID := reader.Int32()

	canInteract, err := s.CanInteractWithMailingObject(ctx, mailBox)
	if err != nil {
		return err
	}
	if !canInteract {
		return fmt.Errorf("player '%d' tried to interact with '%d' object, that not in reach", s.character.GUID, mailBox)
	}

	gameClient, err := s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(s.worldSocket.Address())
	if err != nil {
		return fmt.Errorf("can't get gameServiceClient, err: %w", err)
	}

	removeResult, err := s.mailServiceClient.RemoveMailMoney(ctx, &pbMail.RemoveMailMoneyRequest{
		Api:        root.SupportedMailServiceVer,
		RealmID:    root.RealmID,
		PlayerGuid: &s.character.GUID,
		MailID:     mailID,
	})
	if err != nil {
		return NewMailServiceUnavailableErr(err)
	}

	if removeResult.MoneyRemoved == 0 {
		s.gameSocket.SendPacket(mailResult{
			MailID: uint(mailID),
			Type:   MailResponseTypeMoneyTaken,
			Status: MailResponseStatusInternalError,
		}.BuildPacket())
		return nil
	}

	_, err = gameClient.ModifyMoneyForPlayer(ctx, &pb.ModifyMoneyForPlayerRequest{
		Api:        "",
		PlayerGuid: s.character.GUID,
		Value:      removeResult.MoneyRemoved,
	})
	if err != nil {
		return err
	}

	s.gameSocket.SendPacket(mailResult{
		MailID: uint(mailID),
		Type:   MailResponseTypeMoneyTaken,
		Status: MailResponseStatusOk,
	}.BuildPacket())

	return nil
}

func (s *GameSession) HandleMailTakeItem(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	mailBox := reader.Uint64()
	mailID := reader.Int32()
	itemID := reader.Int32()

	canInteract, err := s.CanInteractWithMailingObject(ctx, mailBox)
	if err != nil {
		return err
	}
	if !canInteract {
		return fmt.Errorf("player '%d' tried to interact with '%d' object, that not in reach", s.character.GUID, mailBox)
	}

	mailResp, err := s.mailServiceClient.MailByID(ctx, &pbMail.MailByIDRequest{
		Api:     root.SupportedMailServiceVer,
		RealmID: root.RealmID,
		MailID:  mailID,
	})
	if err != nil {
		return NewMailServiceUnavailableErr(err)
	}

	var item *pbMail.ItemAttachment
	for _, attachment := range mailResp.Mail.Attachments {
		if attachment.Guid == uint64(itemID) {
			item = attachment
			break
		}
	}

	if item == nil || mailResp.Mail.ReceiverGuid != s.character.GUID {
		s.gameSocket.SendPacket(mailResult{
			MailID: uint(mailID),
			Type:   MailResponseTypeItemTaken,
			Status: MailResponseStatusMailAttachmentInvalid,
		}.BuildPacket())
		return fmt.Errorf("item %d not found in the given mail %d", itemID, mailID)
	}

	if mailResp.Mail.CashOnDelivery > 0 {
		moneyResp, err := s.gameServerGRPCClient.GetMoneyForPlayer(ctx, &pb.GetMoneyForPlayerRequest{
			PlayerGuid: s.character.GUID,
		})
		if err != nil {
			return fmt.Errorf("can't get money for player, err: %w", err)
		}

		if moneyResp.Money < uint32(mailResp.Mail.CashOnDelivery) {
			s.gameSocket.SendPacket(mailResult{
				MailID: uint(mailID),
				Type:   MailResponseTypeItemTaken,
				Status: MailResponseStatusNotEnoughMoney,
			}.BuildPacket())
			return nil
		}

		_, err = s.gameServerGRPCClient.ModifyMoneyForPlayer(ctx, &pb.ModifyMoneyForPlayerRequest{
			PlayerGuid: s.character.GUID,
			Value:      -mailResp.Mail.CashOnDelivery,
		})
		if err != nil {
			return fmt.Errorf("can't modify money for player, err: %w", err)
		}
	}

	addResp, err := s.gameServerGRPCClient.AddExistingItemToPlayer(ctx, &pb.AddExistingItemToPlayerRequest{
		Api:        "",
		PlayerGuid: s.character.GUID,
		Item: &pb.AddExistingItemToPlayerRequest_Item{
			Guid:             guid.NewFromEntryAndCounter(guid.Item, item.Entry, guid.LowType(itemID)).GetRawValue(),
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
		const bagFullErr uint32 = 4
		s.gameSocket.SendPacket(mailResult{
			MailID:    uint(mailID),
			Type:      MailResponseTypeItemTaken,
			Status:    MailResponseStatusEquipErr,
			BagResult: bagFullErr,
		}.BuildPacket())
		return nil
	}

	_, err = s.mailServiceClient.RemoveMailItem(ctx, &pbMail.RemoveMailItemRequest{
		Api:                  root.SupportedMailServiceVer,
		RealmID:              root.RealmID,
		PlayerGuid:           &s.character.GUID,
		MailID:               mailID,
		ItemGuid:             uint64(itemID),
		HandleCashOnDelivery: mailResp.Mail.CashOnDelivery > 0,
	})
	if err != nil {
		return fmt.Errorf("mail service failed to remove mail item, err: %w", err)
	}

	s.gameSocket.SendPacket(mailResult{
		MailID:         uint(mailID),
		Type:           MailResponseTypeItemTaken,
		Status:         MailResponseStatusOk,
		AttachID:       uint32(itemID),
		QtyInInventory: item.Count,
	}.BuildPacket())

	return nil
}

func (s *GameSession) HandleQueryNextMailTime(ctx context.Context, p *packet.Packet) error {
	resp, err := s.mailServiceClient.MailsForPlayer(ctx, &pbMail.MailsForPlayerRequest{
		Api:        root.SupportedMailServiceVer,
		RealmID:    root.RealmID,
		PlayerGuid: s.character.GUID,
	})
	if err != nil {
		return NewMailServiceUnavailableErr(fmt.Errorf("can't fetch mail list, err: %w", err))
	}

	const MailReadFlag = 1
	unreadMails := []*pbMail.Mail{}
	for i, mail := range resp.Mails {
		// Ignore the rest. Otherwise, mail icon can be stuck for player.
		if i == MaxVisibleMailsCount {
			break
		}

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

func (s *GameSession) HandleDeleteMail(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	mailBox := reader.Uint64()
	mailID := reader.Int32()
	//deleteReason := reader.Int32()

	canInteract, err := s.CanInteractWithMailingObject(ctx, mailBox)
	if err != nil {
		return err
	}
	if !canInteract {
		return fmt.Errorf("player '%d' tried to interact with '%d' object, that not in reach", s.character.GUID, mailBox)
	}

	mailResp, err := s.mailServiceClient.MailByID(ctx, &pbMail.MailByIDRequest{
		Api:     root.SupportedMailServiceVer,
		RealmID: root.RealmID,
		MailID:  mailID,
	})
	if err != nil {
		return NewMailServiceUnavailableErr(err)
	}

	if mailResp.Mail.CashOnDelivery > 0 || mailResp.Mail.ReceiverGuid != s.character.GUID {
		s.gameSocket.SendPacket(mailResult{
			MailID: uint(mailID),
			Type:   MailResponseTypeDeleted,
			Status: MailResponseStatusInternalError,
		}.BuildPacket())
		return nil
	}

	_, err = s.mailServiceClient.DeleteMail(ctx, &pbMail.DeleteMailRequest{
		Api:     root.SupportedMailServiceVer,
		RealmID: root.RealmID,
		MailID:  mailID,
	})
	if err != nil {
		return NewMailServiceUnavailableErr(err)
	}

	s.gameSocket.SendPacket(mailResult{
		MailID: uint(mailID),
		Type:   MailResponseTypeDeleted,
		Status: MailResponseStatusOk,
	}.BuildPacket())

	return nil
}

func (s *GameSession) CanInteractWithMailingObject(ctx context.Context, object uint64) (bool, error) {
	switch guid.New(object).GetHigh() {
	case guid.GameObject:
		const gameObjectTypeMailbox = 19
		resp, err := s.gameServerGRPCClient.CanPlayerInteractWithGameObject(ctx, &pb.CanPlayerInteractWithGameObjectRequest{
			Api:            "",
			PlayerGuid:     s.character.GUID,
			GameObjectGuid: object,
			GameObjectType: gameObjectTypeMailbox,
		})
		if err != nil {
			return false, fmt.Errorf("failed to make CanPlayerInteractWithGameObject request, err: %w", err)
		}
		return resp.CanInteract, nil
	case guid.Unit:
		const npcFlagMailbox = 0x04000000
		resp, err := s.gameServerGRPCClient.CanPlayerInteractWithNPC(ctx, &pb.CanPlayerInteractWithNPCRequest{
			Api:        "",
			PlayerGuid: s.character.GUID,
			NpcGuid:    object,
			NpcFlags:   npcFlagMailbox,
		})
		if err != nil {
			return false, fmt.Errorf("failed to make CanPlayerInteractWithNPC request, err: %w", err)
		}
		return resp.CanInteract, nil
	}

	return false, nil
}
