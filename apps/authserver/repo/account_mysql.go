package repo

import (
	"context"
	"database/sql"
)

type accountMySQLRepo struct {
	db *sql.DB

	accountByUserStmt *sql.Stmt
	updateAccountStmt *sql.Stmt
}

func NewAccountMySQLRepo(db *sql.DB, stmtBuilder StatementsBuilder) (AccountRepo, error) {
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
	return account, nil
}

func (r *accountMySQLRepo) UpdateAccount(ctx context.Context, a *Account) error {
	_, err := r.updateAccountStmt.ExecContext(ctx, a.Username, a.Salt, a.Verifier, a.SessionKeyAuth, a.Locked, a.LastIP, a.ID)
	return err
}
