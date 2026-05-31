package repo

import "fmt"

const (
	StmtLoadAllAuctions CharsPreparedStatements = iota
	StmtInsertAuction
	StmtUpdateBid
	StmtDeleteAuction
	StmtLoadAuctionByID
)

// CharsPreparedStatements represents prepared statements for the characters database.
type CharsPreparedStatements uint32

// ID returns identifier of prepared statement.
func (s CharsPreparedStatements) ID() uint32 {
	return uint32(s) + 10000 // offset to avoid collision with mail stmts
}

// Stmt returns prepared statement as string.
func (s CharsPreparedStatements) Stmt() string {
	switch s {
	case StmtLoadAllAuctions:
		return `SELECT ah.id, ah.houseid, ah.itemguid, ah.itemowner,
			ah.buyoutprice, ah.time, ah.buyguid, ah.lastbid,
			ah.startbid, ah.deposit,
			COALESCE(ii.itemEntry, 0), COALESCE(ii.count, 0),
			COALESCE(CAST(SUBSTRING_INDEX(ii.charges, ' ', 1) AS SIGNED), 0),
			COALESCE(ii.randomPropertyId, 0), 0,
			COALESCE(ii.enchantments, ''), COALESCE(ii.flags, 0)
			FROM auctionhouse ah
			LEFT JOIN item_instance ii ON ah.itemguid = ii.guid`
	case StmtInsertAuction:
		return `INSERT INTO auctionhouse (id, houseid, itemguid, itemowner, buyoutprice, time, buyguid, lastbid, startbid, deposit) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	case StmtUpdateBid:
		return `UPDATE auctionhouse SET buyguid = ?, lastbid = ? WHERE id = ?`
	case StmtDeleteAuction:
		return `DELETE FROM auctionhouse WHERE id = ?`
	case StmtLoadAuctionByID:
		return `SELECT ah.id, ah.houseid, ah.itemguid, ah.itemowner,
			ah.buyoutprice, ah.time, ah.buyguid, ah.lastbid,
			ah.startbid, ah.deposit,
			COALESCE(ii.itemEntry, 0), COALESCE(ii.count, 0),
			COALESCE(CAST(SUBSTRING_INDEX(ii.charges, ' ', 1) AS SIGNED), 0),
			COALESCE(ii.randomPropertyId, 0), 0,
			COALESCE(ii.enchantments, ''), COALESCE(ii.flags, 0)
			FROM auctionhouse ah
			LEFT JOIN item_instance ii ON ah.itemguid = ii.guid
			WHERE ah.id = ?`
	}
	panic(fmt.Errorf("unk stmt %d", s))
}