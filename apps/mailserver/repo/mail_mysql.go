package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

type mailMySQLRepo struct {
	db shrepo.CharactersDB
}

func NewMailMySQLRepo(db shrepo.CharactersDB) (MailRepo, error) {
	db.SetPreparedStatement(StmtGetMailsForCharacter)
	db.SetPreparedStatement(StmtGetMailsItemsForCharacter)
	db.SetPreparedStatement(StmtUpdateMailFlagsMask)
	db.SetPreparedStatement(StmtCreateNewMail)
	db.SetPreparedStatement(StmtCreateMailItem)
	db.SetPreparedStatement(StmtGetMailItemsByID)
	db.SetPreparedStatement(StmtGetMailByID)
	db.SetPreparedStatement(StmtDeleteMailItem)
	db.SetPreparedStatement(StmtDeleteMailItemForPlayer)
	db.SetPreparedStatement(StmtUpdateMailByID)
	db.SetPreparedStatement(StmtSelectExpiredMails)
	db.SetPreparedStatement(StmtUpdateMailItemsReceiverByMailID)

	return &mailMySQLRepo{
		db: db,
	}, nil
}

func (m *mailMySQLRepo) AddMail(ctx context.Context, realmID uint32, mail *Mail) error {
	tx, err := m.db.DBByRealm(realmID).Begin()
	if err != nil {
		return err
	}

	mailExec, err := tx.ExecContext(ctx, StmtCreateNewMail.Stmt(),
		mail.Type, mail.Stationery, mail.TemplateID, mail.SenderGuid,
		mail.ReceiverGuid, mail.Subject, mail.Body, len(mail.Attachments) > 0,
		mail.ExpirationTimestamp, mail.DeliveryTimestamp, mail.MoneyToSend,
		mail.CashOnDelivery, mail.FlagsMask)
	if err != nil {
		tx.Rollback()
		return err
	}

	id, err := mailExec.LastInsertId()
	if err != nil {
		return err
	}

	createMailItemStmt, err := tx.Prepare(StmtCreateMailItem.Stmt())
	if err != nil {
		tx.Rollback()
		return err
	}

	createItemInstanceStmt, err := tx.Prepare(StmtUpsertItemInstance.Stmt())
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, att := range mail.Attachments {
		_, err = createMailItemStmt.ExecContext(ctx, id, att.GUID, mail.ReceiverGuid)
		if err != nil {
			tx.Rollback()
			return err
		}

		// TODO: add missing fields.
		_, err = createItemInstanceStmt.ExecContext(ctx,
			att.GUID, att.Entry, att.OwnerGUID, att.Count, 0,
			"0 0 0 0 ", att.Flags, "0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 ",
			att.RandomPropertyID, att.Durability, att.Text,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}

	mail.ID = uint(id)

	return nil
}

func (m *mailMySQLRepo) MailListForPlayer(ctx context.Context, realmID uint32, playerGUID uint64) ([]Mail, error) {
	rowsMail, err := m.db.PreparedStatement(realmID, StmtGetMailsForCharacter).QueryContext(ctx, playerGUID)
	if err != nil {
		return nil, err
	}
	defer rowsMail.Close()

	mails := []Mail{}
	mailsByID := map[uint]int{}

	for rowsMail.Next() {
		mail := Mail{}
		err = rowsMail.Scan(
			&mail.ID, &mail.Type, &mail.SenderGuid, &mail.ReceiverGuid, &mail.Subject, &mail.Body,
			&mail.ExpirationTimestamp, &mail.DeliveryTimestamp, &mail.MoneyToSend,
			&mail.CashOnDelivery, &mail.FlagsMask, &mail.Stationery, &mail.TemplateID, &mail.HasItemAttachments,
		)
		if err != nil {
			return nil, fmt.Errorf("can't create mail object, err: %w", err)
		}
		mails = append(mails, mail)
		mailsByID[mail.ID] = len(mails) - 1
	}

	rowsAttachment, err := m.db.PreparedStatement(realmID, StmtGetMailsItemsForCharacter).QueryContext(ctx, playerGUID)
	if err != nil {
		return nil, err
	}
	defer rowsAttachment.Close()

	for rowsAttachment.Next() {
		var (
			mailID       uint
			enchantments string // TODO: add enchantments support.
			charges      string // TODO: fix charges type.
		)
		attachment := ItemAttachment{}
		err = rowsAttachment.Scan(
			&attachment.Count, &charges, &attachment.Flags, &enchantments, &attachment.RandomPropertyID, &attachment.Durability,
			&attachment.Text, &attachment.GUID, &attachment.Entry, &attachment.OwnerGUID, &mailID,
		)
		if err != nil {
			return nil, fmt.Errorf("can't create mail attachment object, err: %w", err)
		}

		mails[mailsByID[mailID]].Attachments = append(mails[mailsByID[mailID]].Attachments, attachment)
	}

	return mails, nil
}

func (m *mailMySQLRepo) UpdateMailFlagsMaskForPlayer(ctx context.Context, realmID uint32, playerGUID uint64, mailID uint, mask MailFlagMask) error {
	res, err := m.db.PreparedStatement(realmID, StmtUpdateMailFlagsMask).ExecContext(ctx, mask, mailID, playerGUID)
	if err != nil {
		return err
	}

	if rows, err := res.RowsAffected(); rows == 0 {
		if err != nil {
			return err
		}

		return fmt.Errorf("mail with id '%d' and received '%d' not found", mailID, playerGUID)
	}

	return nil
}

func (m *mailMySQLRepo) UpdateMailWithoutAttachments(ctx context.Context, realmID uint32, mail *Mail) error {
	_, err := m.db.PreparedStatement(realmID, StmtUpdateMailByID).ExecContext(
		ctx, mail.Type, mail.SenderGuid, mail.ReceiverGuid, mail.Subject, mail.Body,
		mail.ExpirationTimestamp, mail.DeliveryTimestamp, mail.MoneyToSend,
		mail.CashOnDelivery, mail.FlagsMask, mail.Stationery, mail.TemplateID, mail.ID,
	)
	if err != nil {
		return fmt.Errorf("can't update mail without attachments, err: %w", err)
	}
	return err
}

func (m *mailMySQLRepo) MailByID(ctx context.Context, realmID uint32, mailID uint) (*Mail, error) {
	mail, err := m.mailByIDWithoutAttachments(ctx, realmID, mailID)
	if err != nil {
		return nil, err
	}

	rowsAttachment, err := m.db.PreparedStatement(realmID, StmtGetMailItemsByID).QueryContext(ctx, mailID)
	if err != nil {
		return nil, err
	}
	defer rowsAttachment.Close()

	for rowsAttachment.Next() {
		var (
			mailID       uint
			enchantments string // TODO: add enchantments support.
			charges      string // TODO: fix charges type.
		)
		attachment := ItemAttachment{}
		err = rowsAttachment.Scan(
			&attachment.Count, &charges, &attachment.Flags, &enchantments, &attachment.RandomPropertyID, &attachment.Durability,
			&attachment.Text, &attachment.GUID, &attachment.Entry, &attachment.OwnerGUID, &mailID,
		)
		if err != nil {
			return nil, fmt.Errorf("can't create mail attachment object, err: %w", err)
		}

		mail.Attachments = append(mail.Attachments, attachment)
	}

	return mail, nil
}

func (m *mailMySQLRepo) RemoveMailItem(ctx context.Context, realmID uint32, mailID uint, mailItemGUID uint64) error {
	res, err := m.db.PreparedStatement(realmID, StmtDeleteMailItem).ExecContext(ctx, mailID, mailItemGUID)
	if err != nil {
		return err
	}

	if rows, err := res.RowsAffected(); rows == 0 {
		if err != nil {
			return err
		}

		return fmt.Errorf("mailItem with id '%d' and item id '%d' not found", mailID, mailItemGUID)
	}

	return nil
}

func (m *mailMySQLRepo) RemoveMailItemForPlayer(ctx context.Context, realmID uint32, mailID uint, mailItemGUID, playerGUID uint64) error {
	res, err := m.db.PreparedStatement(realmID, StmtDeleteMailItemForPlayer).ExecContext(ctx, mailID, mailItemGUID, playerGUID)
	if err != nil {
		return err
	}

	if rows, err := res.RowsAffected(); rows == 0 {
		if err != nil {
			return err
		}

		return fmt.Errorf("mailItem with id '%d', item id '%d' and player id '%d' not found", mailID, mailItemGUID, playerGUID)
	}

	return nil
}

func (m *mailMySQLRepo) RemoveMailMoney(ctx context.Context, realmID uint32, mailID uint) (int32, error) {
	mail, err := m.mailByIDWithoutAttachments(ctx, realmID, mailID)
	if err != nil {
		return 0, err
	}

	mailMoney := mail.MoneyToSend

	mail.MoneyToSend = 0
	err = m.updateMailByIDWithoutAttachments(ctx, realmID, mail)
	if err != nil {
		return 0, fmt.Errorf("can't remove money, err: %w", err)
	}

	return mailMoney, nil
}

func (m *mailMySQLRepo) RemoveMailMoneyForPlayer(ctx context.Context, realmID uint32, mailID uint, playerGUID uint64) (int32, error) {
	mail, err := m.mailByIDWithoutAttachments(ctx, realmID, mailID)
	if err != nil {
		return 0, err
	}

	if mail.ReceiverGuid != playerGUID {
		return 0, nil
	}

	mailMoney := mail.MoneyToSend

	mail.MoneyToSend = 0
	err = m.updateMailByIDWithoutAttachments(ctx, realmID, mail)
	if err != nil {
		return 0, fmt.Errorf("can't remove money, err: %w", err)
	}

	return mailMoney, nil
}

func (m *mailMySQLRepo) ExpiredMails(ctx context.Context, realmID uint32) ([]Mail, error) {
	rowsMail, err := m.db.PreparedStatement(realmID, StmtSelectExpiredMails).QueryContext(ctx, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rowsMail.Close()

	mails := []Mail{}

	for rowsMail.Next() {
		mail := Mail{}
		err = rowsMail.Scan(
			&mail.ID, &mail.Type, &mail.SenderGuid, &mail.ReceiverGuid, &mail.Subject, &mail.Body,
			&mail.ExpirationTimestamp, &mail.DeliveryTimestamp, &mail.MoneyToSend,
			&mail.CashOnDelivery, &mail.FlagsMask, &mail.Stationery, &mail.TemplateID, &mail.HasItemAttachments,
		)
		if err != nil {
			return nil, fmt.Errorf("can't create expired mail object, err: %w", err)
		}
		mails = append(mails, mail)
	}

	return mails, err
}

func (m *mailMySQLRepo) DeleteMailsWithoutAttachments(ctx context.Context, realmID uint32, IDs []uint) error {
	_, err := m.db.DBByRealm(realmID).ExecContext(ctx, fmt.Sprintf(StmtDeleteMailsWithIDs.Stmt(), strings.Join(uintsToStrings(IDs), ",")))
	if err != nil {
		return fmt.Errorf("can't delete mails without attachments, err: %w", err)
	}

	return nil
}

func (m *mailMySQLRepo) UpdateMailItemsOwner(ctx context.Context, realmID uint32, mailID uint, newReceiverID uint64) error {
	_, err := m.db.PreparedStatement(realmID, StmtUpdateMailItemsReceiverByMailID).ExecContext(ctx, newReceiverID, mailID)
	if err != nil {
		return fmt.Errorf("can't update mail items receiver, err: %w", err)
	}

	return nil
}

func (m *mailMySQLRepo) MailItemsIDsByMailIDs(ctx context.Context, realmID uint32, IDs []uint) ([]uint64, error) {
	mailItemIDsRes, err := m.db.DBByRealm(realmID).QueryContext(ctx, fmt.Sprintf(StmtSelectMailsItemsIDWithMailIDs.Stmt(), strings.Join(uintsToStrings(IDs), ",")))
	if err != nil {
		return nil, err
	}
	defer mailItemIDsRes.Close()

	mailItemsIDs := []uint64{}

	var mailID uint
	var mailItemID uint64
	for mailItemIDsRes.Next() {
		err = mailItemIDsRes.Scan(&mailID, &mailItemID)
		if err != nil {
			return nil, fmt.Errorf("can't scan mail item, err: %w", err)
		}
		mailItemsIDs = append(mailItemsIDs, mailItemID)
	}

	return mailItemsIDs, nil
}

func (m *mailMySQLRepo) DeleteItemsWithIDs(ctx context.Context, realmID uint32, itemIDs []uint64) error {
	_, err := m.db.DBByRealm(realmID).ExecContext(ctx, fmt.Sprintf(StmtDeleteItemsByIDs.Stmt(), strings.Join(uint64sToStrings(itemIDs), ",")))
	if err != nil {
		return fmt.Errorf("can't delete items, err: %w", err)
	}

	return nil
}

func (m *mailMySQLRepo) DeleteMailItemsWithIDs(ctx context.Context, realmID uint32, itemIDs []uint64) error {
	_, err := m.db.DBByRealm(realmID).ExecContext(ctx, fmt.Sprintf(StmtDeleteMailItemsByItemIDs.Stmt(), strings.Join(uint64sToStrings(itemIDs), ",")))
	if err != nil {
		return fmt.Errorf("can't delete mail items, err: %w", err)
	}

	return nil
}

func (m *mailMySQLRepo) updateMailByIDWithoutAttachments(ctx context.Context, realmID uint32, mail *Mail) error {
	res, err := m.db.PreparedStatement(realmID, StmtUpdateMailByID).ExecContext(
		ctx, mail.Type, mail.SenderGuid, mail.ReceiverGuid,
		mail.Subject, mail.Body, mail.ExpirationTimestamp,
		mail.DeliveryTimestamp, mail.MoneyToSend, mail.CashOnDelivery,
		mail.FlagsMask, mail.Stationery, mail.TemplateID,
		mail.ID,
	)

	if err != nil {
		return fmt.Errorf("can't update mail fields, err: %w", err)
	}

	rowsEffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsEffected == 0 {
		return fmt.Errorf("can't update mail, mail with id '%d' not found", mail.ID)
	}

	return nil
}

func (m *mailMySQLRepo) mailByIDWithoutAttachments(ctx context.Context, realmID uint32, mailID uint) (*Mail, error) {
	rowsMail, err := m.db.PreparedStatement(realmID, StmtGetMailByID).QueryContext(ctx, mailID)
	if err != nil {
		return nil, err
	}
	defer rowsMail.Close()

	mail := Mail{}
	for rowsMail.Next() {
		err = rowsMail.Scan(
			&mail.ID, &mail.Type, &mail.SenderGuid, &mail.ReceiverGuid, &mail.Subject, &mail.Body,
			&mail.ExpirationTimestamp, &mail.DeliveryTimestamp, &mail.MoneyToSend,
			&mail.CashOnDelivery, &mail.FlagsMask, &mail.Stationery, &mail.TemplateID, &mail.HasItemAttachments,
		)
		if err != nil {
			return nil, fmt.Errorf("can't create mail object, err: %w", err)
		}
	}

	if mail.ID == 0 {
		return nil, fmt.Errorf("mail not found")
	}

	return &mail, nil
}

func uint64sToStrings(ints []uint64) []string {
	strIDs := make([]string, len(ints))
	for i, id := range ints {
		strIDs[i] = fmt.Sprintf("%d", id)
	}
	return strIDs
}

func uintsToStrings(ints []uint) []string {
	strIDs := make([]string, len(ints))
	for i, id := range ints {
		strIDs[i] = fmt.Sprintf("%d", id)
	}
	return strIDs
}
