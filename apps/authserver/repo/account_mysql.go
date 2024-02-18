package repo

import (
	"context"
	"database/sql"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
	"github.com/walkline/ToCloud9/shared/slices"
)

type accountMySQLRepo struct {
	db *sql.DB

	accountByUserStmt *sql.Stmt
	updateAccountStmt *sql.Stmt

	schemaType shrepo.SupportedSchemaType
}

func NewAccountMySQLRepo(db *sql.DB, stmtBuilder StatementsBuilder, schemaType shrepo.SupportedSchemaType) (AccountRepo, error) {
	accountByUserStmt, err := db.Prepare(stmtBuilder.StmtForType(AuthStmtTypeGetAccountByUsername))
	if err != nil {
		return nil, err
	}

	updateAccountStmt, err := db.Prepare(stmtBuilder.StmtForType(AuthStmtTypeUpdateAccountByID))
	if err != nil {
		return nil, err
	}

	return &accountMySQLRepo{
		db:                db,
		accountByUserStmt: accountByUserStmt,
		updateAccountStmt: updateAccountStmt,
		schemaType:        schemaType,
	}, nil
}

func (r *accountMySQLRepo) AccountByUserName(ctx context.Context, username string) (*Account, error) {
	account := &Account{}
	row := r.accountByUserStmt.QueryRowContext(ctx, username)
	err := row.Scan(&account.ID, &account.Username, &account.Salt, &account.Verifier, &account.SessionKeyAuth, &account.Locked, &account.LastIP)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if r.schemaType == shrepo.SupportedSchemaTypeCMaNGOS {
		slices.ReverseBytes(account.Verifier)
		slices.ReverseBytes(account.Salt)
		slices.ReverseBytes(account.SessionKeyAuth)
	}

	return account, nil
}

func (r *accountMySQLRepo) UpdateAccount(ctx context.Context, a *Account) error {
	var salt, verifier, sessionKey []byte
	if r.schemaType == shrepo.SupportedSchemaTypeCMaNGOS {
		salt = make([]byte, len(a.Salt))
		verifier = make([]byte, len(a.Verifier))
		sessionKey = make([]byte, len(a.SessionKeyAuth))
		copy(salt, a.Salt)
		copy(verifier, a.Verifier)
		copy(sessionKey, a.SessionKeyAuth)
		slices.ReverseBytes(salt)
		slices.ReverseBytes(verifier)
		slices.ReverseBytes(sessionKey)
	} else {
		salt = a.Salt
		verifier = a.Verifier
		sessionKey = a.SessionKeyAuth
	}
	_, err := r.updateAccountStmt.ExecContext(ctx, a.Username, salt, verifier, sessionKey, a.Locked, a.LastIP, a.ID)
	return err
}
