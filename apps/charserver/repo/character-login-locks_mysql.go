package repo

import (
	"context"
	"database/sql"

	"github.com/go-sql-driver/mysql"
)

type characterLoginLocksMySQL struct{ db *sql.DB }

func NewCharacterLoginLocksMySQL(db *sql.DB) CharacterLoginLocks {
	return &characterLoginLocksMySQL{db: db}
}

func (r *characterLoginLocksMySQL) Acquire(ctx context.Context, realmID, accountID uint32, characterGUID uint64, gatewayID string) (bool, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO character_login_lock (realm_id, account_id, character_guid, gateway_id) VALUES (?, ?, ?, ?)`, realmID, accountID, characterGUID, gatewayID)
	if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
		return false, nil
	}
	return err == nil, err
}

func (r *characterLoginLocksMySQL) Release(ctx context.Context, realmID, accountID uint32, characterGUID uint64, gatewayID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM character_login_lock WHERE realm_id = ? AND account_id = ? AND character_guid = ? AND gateway_id = ?`, realmID, accountID, characterGUID, gatewayID)
	return err
}

func (r *characterLoginLocksMySQL) ReleaseByGateway(ctx context.Context, realmID uint32, gatewayID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM character_login_lock WHERE realm_id = ? AND gateway_id = ?`, realmID, gatewayID)
	return err
}
