package repo

import (
	"context"
	"database/sql"
)

type realmMySQLRepo struct {
	db *sql.DB

	realmsStmt     *sql.Stmt
	charsCountStmt *sql.Stmt
}

func NewRealmMySQLRepo(db *sql.DB, stmtBuilder StatementsBuilder) (RealmRepo, error) {
	realmsStmt, err := db.Prepare(stmtBuilder.StmtForType(AuthStmtTypeGetRealmList))
	if err != nil {
		return nil, err
	}

	charsCountStmt, err := db.Prepare(stmtBuilder.StmtForType(AuthStmtTypeGetCharactersCountOnRealmsByAccount))
	if err != nil {
		return nil, err
	}

	return &realmMySQLRepo{
		db:             db,
		realmsStmt:     realmsStmt,
		charsCountStmt: charsCountStmt,
	}, nil
}

func (r *realmMySQLRepo) LoadRealms(ctx context.Context) ([]Realm, error) {
	rows, err := r.realmsStmt.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Realm{}
	for rows.Next() {
		item := Realm{}
		err = rows.Scan(&item.ID, &item.Name, &item.Icon, &item.Flag, &item.Timezone, &item.AllowedSecurityLevel, &item.GameBuild)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, nil
}

func (r *realmMySQLRepo) CountCharsPerRealmByAccountID(ctx context.Context, accountID uint32) ([]CharsCountInRealm, error) {
	rows, err := r.charsCountStmt.QueryContext(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []CharsCountInRealm{}
	for rows.Next() {
		item := CharsCountInRealm{}
		err = rows.Scan(&item.RealmID, &item.CharCount)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, nil
}
