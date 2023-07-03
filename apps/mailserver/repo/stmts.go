package repo

import "fmt"

const (
	// StmtGetMailsForCharacter returns mails for character.
	StmtGetMailsForCharacter CharsPreparedStatements = iota

	// StmtGetMailsItemsForCharacter returns mails items for character.
	StmtGetMailsItemsForCharacter

	// StmtUpdateMailFlagsMask updates value for checked field.
	StmtUpdateMailFlagsMask

	// StmtCreateNewMail creates new mail entry.
	StmtCreateNewMail

	// StmtCreateMailItem creates mail item entry.
	StmtCreateMailItem

	// StmtUpsertItemInstance inserts new item instance or updates existing one.
	StmtUpsertItemInstance

	// StmtGetMailByID gets mail with given ID.
	StmtGetMailByID

	// StmtGetMailItemsByID returns mail items by mail ID.
	StmtGetMailItemsByID

	// StmtDeleteMailItemForPlayer deletes mail item with given mailID, itemID and playerID.
	StmtDeleteMailItemForPlayer

	// StmtDeleteMailItem deletes mail item with given mailID, itemID.
	StmtDeleteMailItem

	// StmtUpdateMailByID updates values for mail with id.
	StmtUpdateMailByID

	// StmtSelectExpiredMails selects expired mails.
	StmtSelectExpiredMails

	// StmtUpdateMailItemsReceiverByMailID updates owner of mail items by mail ID.
	StmtUpdateMailItemsReceiverByMailID

	// StmtDeleteMailsWithIDs delete mails with given IDs.
	StmtDeleteMailsWithIDs

	// StmtSelectMailsItemsIDWithMailIDs selects mail items IDs for given mail IDs.
	StmtSelectMailsItemsIDWithMailIDs
)

// CharsPreparedStatements represents prepared statements for the characters database.
// Implements sharedrepo.PreparedStatement interface.
type CharsPreparedStatements uint32

// ID returns identifier of prepared statement.
func (s CharsPreparedStatements) ID() uint32 {
	return uint32(s)
}

// Stmt returns prepared statement as string.
func (s CharsPreparedStatements) Stmt() string {
	switch s {
	case StmtGetMailsForCharacter:
		return "SELECT id, messageType, sender, receiver, subject, body, expire_time, deliver_time, money, cod, checked, stationery, mailTemplateId FROM mail WHERE receiver = ? ORDER BY id DESC"
	case StmtGetMailsItemsForCharacter:
		return "SELECT count, charges, flags, enchantments, randomPropertyId, durability, text, item_guid, itemEntry, ii.owner_guid, m.id FROM mail_items mi INNER JOIN mail m ON mi.mail_id = m.id LEFT JOIN item_instance ii ON mi.item_guid = ii.guid WHERE m.receiver = ?"
	case StmtUpdateMailFlagsMask:
		return "UPDATE mail SET checked = ? WHERE id = ? AND receiver = ?"
	case StmtCreateNewMail:
		return "INSERT INTO mail(messageType, stationery, mailTemplateId, sender, receiver, subject, body, has_items, expire_time, deliver_time, money, cod, checked) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	case StmtCreateMailItem:
		return "INSERT INTO mail_items(mail_id, item_guid, receiver) VALUES (?, ?, ?)"
	case StmtUpsertItemInstance:
		return `INSERT INTO item_instance (guid, itemEntry, owner_guid, count, duration, charges, flags, enchantments, randomPropertyId, durability, text)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		    owner_guid = VALUES(owner_guid),
		    count = VALUES(count),
		    duration = VALUES(duration),
		    charges = VALUES(charges),
		    flags = VALUES(flags),
		    enchantments = VALUES(enchantments),
		    randomPropertyId = VALUES(randomPropertyId),
		    durability = VALUES(durability),
 		    text = VALUES(text)`
	case StmtGetMailByID:
		return "SELECT id, messageType, sender, receiver, subject, body, expire_time, deliver_time, money, cod, checked, stationery, mailTemplateId FROM mail WHERE id = ?"
	case StmtGetMailItemsByID:
		return "SELECT count, charges, flags, enchantments, randomPropertyId, durability, text, item_guid, itemEntry, ii.owner_guid, m.id FROM mail_items mi INNER JOIN mail m ON mi.mail_id = m.id LEFT JOIN item_instance ii ON mi.item_guid = ii.guid WHERE m.id = ?"
	case StmtDeleteMailItem:
		return "DELETE FROM mail_items WHERE mail_id = ? AND item_guid = ?"
	case StmtDeleteMailItemForPlayer:
		return "DELETE FROM mail_items WHERE mail_id = ? AND item_guid = ? AND receiver = ?"
	case StmtUpdateMailByID:
		return "UPDATE mail SET messageType = ?, sender = ?, receiver = ?, subject = ?, body = ?, expire_time = ?, deliver_time = ?, money = ?, cod = ?, checked = ?, stationery = ?, mailTemplateId = ? WHERE id = ?"
	case StmtSelectExpiredMails:
		return "SELECT id, messageType, sender, receiver, has_items, expire_time, cod, checked, mailTemplateId FROM mail WHERE expire_time < ?"
	case StmtUpdateMailItemsReceiverByMailID:
		return "UPDATE mail_items SET receiver = ? WHERE mail_id = ?"
	case StmtDeleteMailsWithIDs:
		return "DELETE mail WHERE id IN (?)"
	case StmtSelectMailsItemsIDWithMailIDs:

	}
	panic(fmt.Errorf("unk stmt %d", s))
}
