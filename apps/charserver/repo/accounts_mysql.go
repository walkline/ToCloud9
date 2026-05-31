package repo

import (
	"context"
	"database/sql"

	"github.com/walkline/ToCloud9/shared/authidentity"
)

type accountsMySQL struct {
	db *sql.DB

	accountByEmailStmt       *sql.Stmt
	relationByPairStmt       *sql.Stmt
	insertRelationStmt       *sql.Stmt
	updateRelationStatusStmt *sql.Stmt
	acceptRelationStmt       *sql.Stmt
	updateRelationNoteStmt   *sql.Stmt
	deleteRelationStmt       *sql.Stmt
	acceptedRelationsStmt    *sql.Stmt
}

func NewAccountsMySQL(db *sql.DB) (Accounts, error) {
	stmts := &accountsMySQL{db: db}
	var err error

	if stmts.accountByEmailStmt, err = db.Prepare("SELECT id, username, email FROM account WHERE UPPER(email) = UPPER(?) OR UPPER(reg_mail) = UPPER(?) LIMIT 1"); err != nil {
		return nil, err
	}
	if stmts.relationByPairStmt, err = db.Prepare(`SELECT account_id_low, account_id_high, requester_account_id, status, note_low, note_high
		FROM tc9_realid_friends WHERE account_id_low = ? AND account_id_high = ?`); err != nil {
		return nil, err
	}
	if stmts.insertRelationStmt, err = db.Prepare(`INSERT INTO tc9_realid_friends
		(account_id_low, account_id_high, requester_account_id, status, note_low, note_high, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, UNIX_TIMESTAMP(), UNIX_TIMESTAMP())
		ON DUPLICATE KEY UPDATE updated_at = updated_at`); err != nil {
		return nil, err
	}
	if stmts.updateRelationStatusStmt, err = db.Prepare(`UPDATE tc9_realid_friends
		SET status = ?, note_low = CASE WHEN account_id_low = ? THEN ? ELSE note_low END,
			note_high = CASE WHEN account_id_high = ? THEN ? ELSE note_high END,
			updated_at = UNIX_TIMESTAMP()
		WHERE account_id_low = ? AND account_id_high = ?`); err != nil {
		return nil, err
	}
	if stmts.acceptRelationStmt, err = db.Prepare(`UPDATE tc9_realid_friends
		SET status = ?, note_low = CASE WHEN account_id_low = ? THEN ? ELSE note_low END,
			note_high = CASE WHEN account_id_high = ? THEN ? ELSE note_high END,
			updated_at = UNIX_TIMESTAMP()
		WHERE account_id_low = ? AND account_id_high = ? AND requester_account_id = ? AND status = ?`); err != nil {
		return nil, err
	}
	if stmts.updateRelationNoteStmt, err = db.Prepare(`UPDATE tc9_realid_friends
		SET note_low = CASE WHEN account_id_low = ? THEN ? ELSE note_low END,
			note_high = CASE WHEN account_id_high = ? THEN ? ELSE note_high END,
			updated_at = UNIX_TIMESTAMP()
		WHERE account_id_low = ? AND account_id_high = ?`); err != nil {
		return nil, err
	}
	if stmts.deleteRelationStmt, err = db.Prepare("DELETE FROM tc9_realid_friends WHERE account_id_low = ? AND account_id_high = ?"); err != nil {
		return nil, err
	}
	if stmts.acceptedRelationsStmt, err = db.Prepare(`SELECT account_id_low, account_id_high, requester_account_id, status, note_low, note_high
		FROM tc9_realid_friends
		WHERE status = ? AND (account_id_low = ? OR account_id_high = ?)
		ORDER BY updated_at DESC`); err != nil {
		return nil, err
	}

	return stmts, nil
}

func (a *accountsMySQL) AccountByEmail(ctx context.Context, email string) (*Account, error) {
	if !authidentity.IsValidEmail(email) {
		return nil, nil
	}
	email = authidentity.NormalizeLoginIdentity(email)

	account := &Account{}
	if err := a.accountByEmailStmt.QueryRowContext(ctx, email, email).Scan(&account.ID, &account.Username, &account.Email); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return account, nil
}

func (a *accountsMySQL) RequestRealIDFriend(ctx context.Context, requesterAccountID uint32, addresseeAccountID uint32, note string) (*RealIDFriendRelation, error) {
	low, high := realIDPair(requesterAccountID, addresseeAccountID)

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	relation, err := a.realIDRelationByPairTx(ctx, tx, low, high, requesterAccountID)
	if err != nil {
		return nil, err
	}

	if relation == nil {
		noteLow, noteHigh := "", ""
		if requesterAccountID == low {
			noteLow = note
		} else {
			noteHigh = note
		}
		if _, err = tx.StmtContext(ctx, a.insertRelationStmt).ExecContext(ctx, low, high, requesterAccountID, RealIDFriendStatusPending, noteLow, noteHigh); err != nil {
			return nil, err
		}
		relation, err = a.realIDRelationByPairTx(ctx, tx, low, high, requesterAccountID)
		if err != nil {
			return nil, err
		}
	}

	if relation != nil && relation.Status == RealIDFriendStatusPending && relation.RequesterAccountID != requesterAccountID {
		if _, err = tx.StmtContext(ctx, a.updateRelationStatusStmt).ExecContext(ctx, RealIDFriendStatusAccepted, requesterAccountID, note, requesterAccountID, note, low, high); err != nil {
			return nil, err
		}
		relation.Status = RealIDFriendStatusAccepted
		relation.Note = note
	} else if relation != nil && note != "" {
		if _, err = tx.StmtContext(ctx, a.updateRelationNoteStmt).ExecContext(ctx, requesterAccountID, note, requesterAccountID, note, low, high); err != nil {
			return nil, err
		}
		relation.Note = note
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return relation, nil
}

func (a *accountsMySQL) AcceptRealIDFriend(ctx context.Context, accountID uint32, requesterAccountID uint32, note string) (*RealIDFriendRelation, error) {
	low, high := realIDPair(accountID, requesterAccountID)

	relation, err := a.realIDRelationByPair(ctx, low, high, accountID)
	if err != nil || relation == nil {
		return relation, err
	}
	if relation.RequesterAccountID != requesterAccountID {
		return nil, nil
	}
	if relation.Status == RealIDFriendStatusAccepted {
		return relation, nil
	}
	if relation.Status != RealIDFriendStatusPending || accountID == requesterAccountID {
		return nil, nil
	}

	if _, err := a.acceptRelationStmt.ExecContext(ctx, RealIDFriendStatusAccepted, accountID, note, accountID, note, low, high, requesterAccountID, RealIDFriendStatusPending); err != nil {
		return nil, err
	}
	return a.realIDRelationByPair(ctx, low, high, accountID)
}

func (a *accountsMySQL) RemoveRealIDFriend(ctx context.Context, accountID uint32, friendAccountID uint32) error {
	low, high := realIDPair(accountID, friendAccountID)
	_, err := a.deleteRelationStmt.ExecContext(ctx, low, high)
	return err
}

func (a *accountsMySQL) UpdateRealIDFriendNote(ctx context.Context, accountID uint32, friendAccountID uint32, note string) error {
	low, high := realIDPair(accountID, friendAccountID)
	_, err := a.updateRelationNoteStmt.ExecContext(ctx, accountID, note, accountID, note, low, high)
	return err
}

func (a *accountsMySQL) AcceptedRealIDFriends(ctx context.Context, accountID uint32) ([]*RealIDFriendRelation, error) {
	rows, err := a.acceptedRelationsStmt.QueryContext(ctx, RealIDFriendStatusAccepted, accountID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*RealIDFriendRelation
	for rows.Next() {
		relation, err := scanRealIDRelation(rows, accountID)
		if err != nil {
			return nil, err
		}
		result = append(result, relation)
	}
	return result, rows.Err()
}

func (a *accountsMySQL) realIDRelationByPair(ctx context.Context, low uint32, high uint32, accountID uint32) (*RealIDFriendRelation, error) {
	return scanRealIDRelationRow(a.relationByPairStmt.QueryRowContext(ctx, low, high), accountID)
}

func (a *accountsMySQL) realIDRelationByPairTx(ctx context.Context, tx *sql.Tx, low uint32, high uint32, accountID uint32) (*RealIDFriendRelation, error) {
	return scanRealIDRelationRow(tx.StmtContext(ctx, a.relationByPairStmt).QueryRowContext(ctx, low, high), accountID)
}

func scanRealIDRelationRow(row *sql.Row, accountID uint32) (*RealIDFriendRelation, error) {
	var low, high, requester uint32
	var status uint8
	var noteLow, noteHigh string
	if err := row.Scan(&low, &high, &requester, &status, &noteLow, &noteHigh); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return realIDRelationFromRow(accountID, low, high, requester, status, noteLow, noteHigh), nil
}

type realIDRelationScanner interface {
	Scan(dest ...interface{}) error
}

func scanRealIDRelation(row realIDRelationScanner, accountID uint32) (*RealIDFriendRelation, error) {
	var low, high, requester uint32
	var status uint8
	var noteLow, noteHigh string
	if err := row.Scan(&low, &high, &requester, &status, &noteLow, &noteHigh); err != nil {
		return nil, err
	}
	return realIDRelationFromRow(accountID, low, high, requester, status, noteLow, noteHigh), nil
}

func realIDRelationFromRow(accountID uint32, low uint32, high uint32, requester uint32, status uint8, noteLow string, noteHigh string) *RealIDFriendRelation {
	friendAccountID := low
	note := noteHigh
	if accountID == low {
		friendAccountID = high
		note = noteLow
	}

	return &RealIDFriendRelation{
		AccountID:          accountID,
		FriendAccountID:    friendAccountID,
		RequesterAccountID: requester,
		Status:             status,
		Note:               note,
	}
}

func realIDPair(a uint32, b uint32) (uint32, uint32) {
	if a < b {
		return a, b
	}
	return b, a
}
