package repo

import (
	"context"
	"fmt"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

type auctionMySQLRepo struct {
	db shrepo.CharactersDB
}

func NewAuctionMySQLRepo(db shrepo.CharactersDB) (AuctionRepo, error) {
	db.SetPreparedStatement(StmtLoadAllAuctions)
	db.SetPreparedStatement(StmtInsertAuction)
	db.SetPreparedStatement(StmtUpdateBid)
	db.SetPreparedStatement(StmtDeleteAuction)
	db.SetPreparedStatement(StmtLoadAuctionByID)

	return &auctionMySQLRepo{db: db}, nil
}

func (r *auctionMySQLRepo) LoadAllAuctions(ctx context.Context, realmID uint32) ([]AuctionEntry, error) {
	rows, err := r.db.PreparedStatement(realmID, StmtLoadAllAuctions).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't load auctions: %w", err)
	}
	defer rows.Close()

	var auctions []AuctionEntry
	for rows.Next() {
		var a AuctionEntry
		err = rows.Scan(
			&a.ID, &a.HouseID, &a.ItemGUID, &a.ItemOwner,
			&a.BuyoutPrice, &a.Time, &a.BuyGUID, &a.LastBid,
			&a.StartBid, &a.Deposit,
			&a.ItemEntry, &a.ItemCount, &a.Charges,
			&a.RandomPropertyID, &a.SuffixFactor,
			&a.Enchantments, &a.Flags,
		)
		if err != nil {
			return nil, fmt.Errorf("can't scan auction: %w", err)
		}
		auctions = append(auctions, a)
	}

	return auctions, nil
}

func (r *auctionMySQLRepo) InsertAuction(ctx context.Context, realmID uint32, a *AuctionEntry) error {
	_, err := r.db.PreparedStatement(realmID, StmtInsertAuction).ExecContext(ctx,
		a.ID, a.HouseID, a.ItemGUID, a.ItemOwner,
		a.BuyoutPrice, a.Time, a.BuyGUID, a.LastBid,
		a.StartBid, a.Deposit,
	)
	if err != nil {
		return fmt.Errorf("can't insert auction: %w", err)
	}
	return nil
}

func (r *auctionMySQLRepo) UpdateBid(ctx context.Context, realmID uint32, auctionID, buyGUID, lastBid uint32) error {
	_, err := r.db.PreparedStatement(realmID, StmtUpdateBid).ExecContext(ctx, buyGUID, lastBid, auctionID)
	if err != nil {
		return fmt.Errorf("can't update bid: %w", err)
	}
	return nil
}

func (r *auctionMySQLRepo) DeleteAuction(ctx context.Context, realmID uint32, auctionID uint32) error {
	_, err := r.db.PreparedStatement(realmID, StmtDeleteAuction).ExecContext(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("can't delete auction: %w", err)
	}
	return nil
}

func (r *auctionMySQLRepo) DeleteAuctionIfExists(ctx context.Context, realmID uint32, auctionID uint32) (*AuctionEntry, bool) {
	// First, try to load the auction
	row := r.db.PreparedStatement(realmID, StmtLoadAuctionByID).QueryRowContext(ctx, auctionID)

	var a AuctionEntry
	err := row.Scan(
		&a.ID, &a.HouseID, &a.ItemGUID, &a.ItemOwner,
		&a.BuyoutPrice, &a.Time, &a.BuyGUID, &a.LastBid,
		&a.StartBid, &a.Deposit,
		&a.ItemEntry, &a.ItemCount, &a.Charges,
		&a.RandomPropertyID, &a.SuffixFactor,
		&a.Enchantments, &a.Flags,
	)
	if err != nil {
		// Auction doesn't exist (already deleted by another instance)
		return nil, false
	}

	// Now try to delete it
	result, err := r.db.PreparedStatement(realmID, StmtDeleteAuction).ExecContext(ctx, auctionID)
	if err != nil {
		return nil, false
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		// Auction was deleted by another instance between SELECT and DELETE
		return nil, false
	}

	// We successfully deleted it
	return &a, true
}