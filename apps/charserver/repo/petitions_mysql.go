package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

type PetitionsPreparedStatements uint32

func (s PetitionsPreparedStatements) Stmt() string {
	switch s {
	case StmtGetPetitionByItemLow:
		return "SELECT ownerguid, petitionguid, name, type FROM petition WHERE petitionguid = ?"
	case StmtGetSignaturesByItemLow:
		return "SELECT playerguid, player_account FROM petition_sign WHERE petitionguid = ?"
	case StmtAddPetitionSignature:
		return "INSERT INTO petition_sign (ownerguid, petitionguid, playerguid, player_account, type) VALUES (?, ?, ?, ?, ?)"
	case StmtRenamePetition:
		return "UPDATE petition SET name = ? WHERE petitionguid = ?"
	case StmtDeletePetitionByItemLow:
		return "DELETE FROM petition WHERE petitionguid = ?"
	case StmtDeletePetitionSignaturesByItemLow:
		return "DELETE FROM petition_sign WHERE petitionguid = ?"
	case StmtCharacterBriefByGUID:
		return "SELECT c.guid, c.account, c.race, IFNULL(gm.guildid, 0), c.name FROM characters AS c LEFT JOIN guild_member AS gm ON c.guid = gm.guid WHERE c.guid = ? AND c.deleteInfos_Name IS NULL"
	case StmtHasGuildInvite:
		return "SELECT guildId FROM guild_invites WHERE charGuid = ?"
	case StmtDeleteGuildPetitionByOwner:
		return "DELETE FROM petition WHERE ownerguid = ? AND type = ?"
	case StmtDeleteGuildPetitionSignsByOwner:
		return "DELETE FROM petition_sign WHERE ownerguid = ? AND type = ?"
	case StmtInsertPetition:
		return "INSERT INTO petition (ownerguid, petitionguid, name, type) VALUES (?, ?, ?, ?)"
	case StmtGuildExistsByName:
		return "SELECT guildid FROM guild WHERE name = ? LIMIT 1"
	}
	panic(fmt.Errorf("unk petition stmt %d", s))
}

func (s PetitionsPreparedStatements) ID() uint32 {
	// Offset avoids collision with CharactersPreparedStatements on the same CharactersDB.
	return uint32(s) + 1000
}

const (
	StmtGetPetitionByItemLow PetitionsPreparedStatements = iota
	StmtGetSignaturesByItemLow
	StmtAddPetitionSignature
	StmtRenamePetition
	StmtDeletePetitionByItemLow
	StmtDeletePetitionSignaturesByItemLow
	StmtCharacterBriefByGUID
	StmtHasGuildInvite
	StmtDeleteGuildPetitionByOwner
	StmtDeleteGuildPetitionSignsByOwner
	StmtInsertPetition
	StmtGuildExistsByName
)

type PetitionsMYSQL struct {
	db shrepo.CharactersDB
}

func NewPetitionsMYSQL(db shrepo.CharactersDB) Petitions {
	db.SetPreparedStatement(StmtGetPetitionByItemLow)
	db.SetPreparedStatement(StmtGetSignaturesByItemLow)
	db.SetPreparedStatement(StmtAddPetitionSignature)
	db.SetPreparedStatement(StmtRenamePetition)
	db.SetPreparedStatement(StmtDeletePetitionByItemLow)
	db.SetPreparedStatement(StmtDeletePetitionSignaturesByItemLow)
	db.SetPreparedStatement(StmtCharacterBriefByGUID)
	db.SetPreparedStatement(StmtHasGuildInvite)
	db.SetPreparedStatement(StmtDeleteGuildPetitionByOwner)
	db.SetPreparedStatement(StmtDeleteGuildPetitionSignsByOwner)
	db.SetPreparedStatement(StmtInsertPetition)
	db.SetPreparedStatement(StmtGuildExistsByName)
	return &PetitionsMYSQL{db: db}
}

func (p *PetitionsMYSQL) GetPetitionByItemLow(ctx context.Context, realmID, itemLow uint32) (*Petition, error) {
	row := p.db.PreparedStatement(realmID, StmtGetPetitionByItemLow).QueryRowContext(ctx, itemLow)
	var (
		ownerLow    uint32
		petitionLow uint32
		name        string
		typ         uint8
	)
	err := row.Scan(&ownerLow, &petitionLow, &name, &typ)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Petition{
		ItemLow:   petitionLow,
		OwnerGUID: uint64(ownerLow),
		Name:      name,
		Type:      typ,
	}, nil
}

func (p *PetitionsMYSQL) GetSignaturesByItemLow(ctx context.Context, realmID, itemLow uint32) ([]PetitionSignature, error) {
	rows, err := p.db.PreparedStatement(realmID, StmtGetSignaturesByItemLow).QueryContext(ctx, itemLow)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PetitionSignature
	for rows.Next() {
		var playerLow, accountID uint32
		if err = rows.Scan(&playerLow, &accountID); err != nil {
			return nil, err
		}
		result = append(result, PetitionSignature{
			PlayerGUID: uint64(playerLow),
			AccountID:  accountID,
		})
	}
	return result, rows.Err()
}

func (p *PetitionsMYSQL) RenamePetition(ctx context.Context, realmID, itemLow uint32, name string) error {
	_, err := p.db.PreparedStatement(realmID, StmtRenamePetition).ExecContext(ctx, name, itemLow)
	return err
}

func (p *PetitionsMYSQL) DeletePetition(ctx context.Context, realmID, itemLow uint32) error {
	if _, err := p.db.PreparedStatement(realmID, StmtDeletePetitionSignaturesByItemLow).ExecContext(ctx, itemLow); err != nil {
		return err
	}
	_, err := p.db.PreparedStatement(realmID, StmtDeletePetitionByItemLow).ExecContext(ctx, itemLow)
	return err
}

func (p *PetitionsMYSQL) CharacterBriefByGUID(ctx context.Context, realmID uint32, charGUID uint64) (*CharacterBrief, error) {
	row := p.db.PreparedStatement(realmID, StmtCharacterBriefByGUID).QueryRowContext(ctx, charGUID)
	var brief CharacterBrief
	err := row.Scan(&brief.GUID, &brief.AccountID, &brief.Race, &brief.GuildID, &brief.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &brief, nil
}

func (p *PetitionsMYSQL) HasGuildInvite(ctx context.Context, realmID uint32, charGUID uint64) (bool, error) {
	row := p.db.PreparedStatement(realmID, StmtHasGuildInvite).QueryRowContext(ctx, charGUID)
	var guildID uint64
	err := row.Scan(&guildID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		// guild_invites is optional on vanilla character DBs.
		if isMySQLUnknownTable(err) {
			return false, nil
		}
		return false, err
	}
	return guildID != 0, nil
}

func isMySQLUnknownTable(err error) bool {
	// MySQL ER_NO_SUCH_TABLE = 1146.
	msg := err.Error()
	return strings.Contains(msg, "1146") || strings.Contains(msg, "doesn't exist")
}

func (p *PetitionsMYSQL) UpsertGuildPetition(ctx context.Context, realmID uint32, pet *Petition) error {
	if _, err := p.db.PreparedStatement(realmID, StmtDeleteGuildPetitionSignsByOwner).ExecContext(ctx, pet.OwnerGUID, GuildCharterType); err != nil {
		return err
	}
	if _, err := p.db.PreparedStatement(realmID, StmtDeleteGuildPetitionByOwner).ExecContext(ctx, pet.OwnerGUID, GuildCharterType); err != nil {
		return err
	}
	if _, err := p.db.PreparedStatement(realmID, StmtDeletePetitionSignaturesByItemLow).ExecContext(ctx, pet.ItemLow); err != nil {
		return err
	}
	if _, err := p.db.PreparedStatement(realmID, StmtDeletePetitionByItemLow).ExecContext(ctx, pet.ItemLow); err != nil {
		return err
	}
	_, err := p.db.PreparedStatement(realmID, StmtInsertPetition).ExecContext(ctx,
		pet.OwnerGUID, pet.ItemLow, pet.Name, GuildCharterType,
	)
	return err
}

func (p *PetitionsMYSQL) AddSignatureIfUnderLimit(ctx context.Context, realmID uint32, ownerLow, itemLow, playerLow, accountID, maxSigns uint32) (bool, bool, error) {
	lockName := fmt.Sprintf("tc9_sign_petition_%d", itemLow)
	db := p.db.DBByRealm(realmID)
	if db == nil {
		return false, false, fmt.Errorf("no characters db for realm %d", realmID)
	}
	var got sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT GET_LOCK(?, 10)", lockName).Scan(&got); err != nil {
		return false, false, err
	}
	if !got.Valid || got.Int64 != 1 {
		return false, false, fmt.Errorf("could not acquire petition sign lock")
	}
	defer func() {
		_, _ = db.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", lockName)
	}()

	rows, err := db.QueryContext(ctx, "SELECT playerguid, player_account FROM petition_sign WHERE petitionguid = ?", itemLow)
	if err != nil {
		return false, false, err
	}
	var n uint32
	for rows.Next() {
		var existingPlayer, existingAccount uint32
		if err = rows.Scan(&existingPlayer, &existingAccount); err != nil {
			rows.Close()
			return false, false, err
		}
		n++
		if existingPlayer == playerLow || existingAccount == accountID {
			rows.Close()
			return false, true, nil
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return false, false, err
	}
	rows.Close()

	if n+1 > maxSigns {
		return false, false, nil
	}

	if _, err := p.db.PreparedStatement(realmID, StmtAddPetitionSignature).ExecContext(ctx,
		ownerLow, itemLow, playerLow, accountID, GuildCharterType,
	); err != nil {
		return false, false, err
	}
	return true, false, nil
}

func (p *PetitionsMYSQL) GuildNameExists(ctx context.Context, realmID uint32, name string) (bool, error) {
	row := p.db.PreparedStatement(realmID, StmtGuildExistsByName).QueryRowContext(ctx, name)
	var guildID uint64
	err := row.Scan(&guildID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return guildID != 0, nil
}
