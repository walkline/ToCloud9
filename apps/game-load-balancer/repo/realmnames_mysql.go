package repo

import (
	"context"
	"database/sql"
)

type realmNamesMySQLRepo struct {
	db *sql.DB
}

func NewRealmNamesMySQLRepo(db *sql.DB) RealmNamesRepo {
	return &realmNamesMySQLRepo{
		db: db,
	}
}

func (r realmNamesMySQLRepo) LoadRealmNames(ctx context.Context) ([]*RealmName, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name FROM realmlist")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	realmNames := make([]*RealmName, 0)
	for rows.Next() {
		realmName := new(RealmName)
		err = rows.Scan(&realmName.RealmID, &realmName.Name)
		if err != nil {
			return nil, err
		}
		realmNames = append(realmNames, realmName)
	}

	return realmNames, nil
}
