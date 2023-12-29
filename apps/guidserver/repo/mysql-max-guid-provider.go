package repo

import (
	"context"
	"fmt"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

type MaxGuidProvider interface {
	// MaxGuidForCharacters returns max guid for characters.
	MaxGuidForCharacters(ctx context.Context, realmID uint32) (uint64, error)

	// MaxGuidForItems returns max guid for items.
	MaxGuidForItems(ctx context.Context, realmID uint32) (uint64, error)

	// MaxGuidForInstances returns max guid for dungeon/raid instance.
	MaxGuidForInstances(ctx context.Context, realmID uint32) (uint64, error)
}

type mysqlMaxGuidRepo struct {
	charDB shrepo.CharactersDB
}

func NewMysqlMaxGuidRepo(db shrepo.CharactersDB) (MaxGuidProvider, error) {
	db.SetPreparedStatement(StmtGetMaxCharacterGUID)
	db.SetPreparedStatement(StmtGetMaxItemGUID)
	db.SetPreparedStatement(StmtGetMaxInstanceGUID)

	return &mysqlMaxGuidRepo{
		charDB: db,
	}, nil
}

func (m *mysqlMaxGuidRepo) MaxGuidForCharacters(ctx context.Context, realmID uint32) (uint64, error) {
	row := m.charDB.PreparedStatement(realmID, StmtGetMaxCharacterGUID).QueryRowContext(ctx)
	if row.Err() != nil {
		return 0, row.Err()
	}

	var guid uint64
	err := row.Scan(&guid)
	if err != nil {
		return 0, err
	}
	return guid, nil
}

func (m *mysqlMaxGuidRepo) MaxGuidForItems(ctx context.Context, realmID uint32) (uint64, error) {
	row := m.charDB.PreparedStatement(realmID, StmtGetMaxItemGUID).QueryRowContext(ctx)
	if row.Err() != nil {
		return 0, row.Err()
	}

	var guid uint64
	err := row.Scan(&guid)
	if err != nil {
		return 0, err
	}
	return guid, nil
}

func (m *mysqlMaxGuidRepo) MaxGuidForInstances(ctx context.Context, realmID uint32) (uint64, error) {
	row := m.charDB.PreparedStatement(realmID, StmtGetMaxInstanceGUID).QueryRowContext(ctx)
	if row.Err() != nil {
		return 0, row.Err()
	}

	var guid uint64
	err := row.Scan(&guid)
	if err != nil {
		return 0, err
	}
	return guid, nil
}

// CharsPreparedStatements represents prepared statements for the characters database.
// Implements sharedrepo.PreparedStatement interface.
type CharsPreparedStatements uint32

const (
	// StmtGetMaxCharacterGUID returns max GUID for characters table.
	StmtGetMaxCharacterGUID CharsPreparedStatements = iota

	// StmtGetMaxItemGUID returns max GUID for item_instance table.
	StmtGetMaxItemGUID

	// StmtGetMaxInstanceGUID returns max GUID for instance table.
	StmtGetMaxInstanceGUID
)

// ID returns identifier of prepared statement.
func (s CharsPreparedStatements) ID() uint32 {
	return uint32(s)
}

// Stmt returns prepared statement as string.
func (s CharsPreparedStatements) Stmt() string {
	switch s {
	case StmtGetMaxCharacterGUID:
		return "SELECT COALESCE(MAX(guid), 0) FROM characters"
	case StmtGetMaxItemGUID:
		return "SELECT COALESCE(MAX(guid), 0) FROM item_instance"
	case StmtGetMaxInstanceGUID:
		return "SELECT COALESCE(MAX(id), 0) FROM instance"
	}
	panic(fmt.Errorf("unk stmt %d", s))
}
