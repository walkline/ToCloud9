package repo

import (
	"database/sql"
	"fmt"
)

// PreparedStatement represents prepared statement with id.
type PreparedStatement interface {
	// ID returns identifier of prepared statement.
	ID() uint32

	// Stmt returns prepared statement as string.
	Stmt() string
}

type CharactersDB interface {
	DBByRealm(realmID uint32) *sql.DB
	SetDBForRealm(realmID uint32, db *sql.DB)

	PreparedStatement(realm uint32, stmt PreparedStatement) *sql.Stmt
	SetPreparedStatement(stmt PreparedStatement)
}

func NewCharactersDB() CharactersDB {
	return &characterDBImpl{
		dbByReam: map[uint32]dbWithPreparedStmts{},
	}
}

type dbWithPreparedStmts struct {
	db    *sql.DB
	stmts map[uint32]*sql.Stmt
}

type characterDBImpl struct {
	// TODO: make thread safe
	dbByReam map[uint32]dbWithPreparedStmts
}

func (c characterDBImpl) DBByRealm(realmID uint32) *sql.DB {
	return c.dbByReam[realmID].db
}

func (c *characterDBImpl) SetDBForRealm(realmID uint32, db *sql.DB) {
	c.dbByReam[realmID] = dbWithPreparedStmts{
		db:    db,
		stmts: map[uint32]*sql.Stmt{},
	}
}

func (c characterDBImpl) PreparedStatement(realm uint32, stmt PreparedStatement) *sql.Stmt {
	return c.dbByReam[realm].stmts[stmt.ID()]
}

func (c *characterDBImpl) SetPreparedStatement(stmt PreparedStatement) {
	for i := range c.dbByReam {
		s, err := c.dbByReam[i].db.Prepare(stmt.Stmt())
		if err != nil {
			panic(fmt.Errorf("can't create prepared stmt with id %d, err: %w", stmt, err))
		}
		c.dbByReam[i].stmts[stmt.ID()] = s
	}
}
