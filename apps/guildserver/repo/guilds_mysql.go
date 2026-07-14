package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

type guildsMySQLRepo struct {
	db shrepo.CharactersDB
}

func NewGuildsMySQLRepo(db shrepo.CharactersDB) (GuildsRepo, error) {
	db.SetPreparedStatement(StmtGetGuildInvite)
	db.SetPreparedStatement(StmtAddGuildInvite)
	db.SetPreparedStatement(StmtRemoveGuildInvite)
	db.SetPreparedStatement(StmtAddGuildMember)
	db.SetPreparedStatement(StmtRemoveGuildMember)
	db.SetPreparedStatement(StmtGetGuildIDByMemberGUID)
	db.SetPreparedStatement(StmtUpdateGuildMessageOfTheDay)
	db.SetPreparedStatement(StmtUpdateGuildMemberPublicNote)
	db.SetPreparedStatement(StmtUpdateGuildMemberOfficersNote)
	db.SetPreparedStatement(StmtUpdateGuildMemberRank)
	db.SetPreparedStatement(StmtUpdateGuildInfo)
	db.SetPreparedStatement(StmtUpdateGuildRank)
	db.SetPreparedStatement(StmtAddGuildRank)
	db.SetPreparedStatement(StmtDeleteGuildRank)

	return &guildsMySQLRepo{
		db: db,
	}, nil
}

// LoadAllForRealm loads all guilds for realm.
// Can be time consuming, better to use it on startup to warmup cache.
func (g *guildsMySQLRepo) LoadAllForRealm(ctx context.Context, realmID uint32) (map[uint64]*Guild, error) {
	// load guilds itself
	rows, err := g.db.DBByRealm(realmID).Query(`
SELECT 	
	g.guildid, g.name, g.leaderguid, g.EmblemStyle, g.EmblemColor, g.BorderStyle, 
	g.BorderColor, g.BackgroundColor, g.info, g.motd, g.createdate, g.BankMoney 
FROM guild g 
ORDER BY g.guildid ASC`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	result := map[uint64]*Guild{}
	for rows.Next() {
		guild := Guild{}
		err = rows.Scan(
			&guild.ID, &guild.Name, &guild.LeaderGUID, &guild.Emblem.Style, &guild.Emblem.Color,
			&guild.Emblem.BorderStyle, &guild.Emblem.BorderColor, &guild.Emblem.BackgroundColor,
			&guild.Info, &guild.MessageOfTheDay, &guild.CrateTimeUnix, &guild.BankMoney,
		)
		if err != nil {
			return nil, err
		}

		result[guild.ID] = &guild
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	// load guild ranks
	rows, err = g.db.DBByRealm(realmID).Query("SELECT guildid, rid, rname, rights, BankMoneyPerDay FROM guild_rank ORDER BY guildid ASC, rid ASC")
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		rank := GuildRank{}
		err = rows.Scan(&rank.GuildID, &rank.Rank, &rank.Name, &rank.Rights, &rank.MoneyPerDay)
		if err != nil {
			return nil, err
		}

		guild := result[rank.GuildID]
		if guild != nil {
			guild.GuildRanks = append(guild.GuildRanks, rank)
		}
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	// load guild members
	rows, err = g.db.DBByRealm(realmID).Query(`
SELECT 
	guildid, gm.guid, ` + "`rank`" + `, pnote, offnote, c.name, c.level, c.class, c.gender, c.zone, c.account, c.online, c.logout_time
FROM guild_member gm
LEFT JOIN characters c ON c.guid = gm.guid ORDER BY guildid ASC`)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		member := GuildMember{}
		err = rows.Scan(
			&member.GuildID, &member.PlayerGUID, &member.Rank, &member.PublicNote, &member.OfficerNote, &member.Name,
			&member.Lvl, &member.Class, &member.Gender, &member.AreaID, &member.Account, &member.Status, &member.LogoutTime,
		)
		if err != nil {
			return nil, err
		}

		guild := result[member.GuildID]
		if guild != nil {
			guild.GuildMembers = append(guild.GuildMembers, &member)
		}
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return result, nil
}

// GuildByRealmAndID loads guild by realm and id.
func (g *guildsMySQLRepo) GuildByRealmAndID(ctx context.Context, realmID uint32, guildID uint64) (*Guild, error) {
	db := g.db.DBByRealm(realmID)

	guild := Guild{RealmID: realmID}
	err := db.QueryRowContext(ctx, `
SELECT 	
	g.guildid, g.name, g.leaderguid, g.EmblemStyle, g.EmblemColor, g.BorderStyle, 
	g.BorderColor, g.BackgroundColor, g.info, g.motd, g.createdate, g.BankMoney 
FROM guild g 
WHERE g.guildid = ?`, guildID).Scan(
		&guild.ID, &guild.Name, &guild.LeaderGUID, &guild.Emblem.Style, &guild.Emblem.Color,
		&guild.Emblem.BorderStyle, &guild.Emblem.BorderColor, &guild.Emblem.BackgroundColor,
		&guild.Info, &guild.MessageOfTheDay, &guild.CrateTimeUnix, &guild.BankMoney,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, "SELECT guildid, rid, rname, rights, BankMoneyPerDay FROM guild_rank WHERE guildid = ? ORDER BY rid ASC", guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		rank := GuildRank{}
		if err = rows.Scan(&rank.GuildID, &rank.Rank, &rank.Name, &rank.Rights, &rank.MoneyPerDay); err != nil {
			return nil, err
		}
		guild.GuildRanks = append(guild.GuildRanks, rank)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	memberRows, err := db.QueryContext(ctx, `
SELECT 
	guildid, gm.guid, `+"`rank`"+`, pnote, offnote, c.name, c.level, c.class, c.gender, c.zone, c.account, c.online, c.logout_time
FROM guild_member gm
LEFT JOIN characters c ON c.guid = gm.guid WHERE gm.guildid = ?`, guildID)
	if err != nil {
		return nil, err
	}
	defer memberRows.Close()

	for memberRows.Next() {
		member := GuildMember{}
		if err = memberRows.Scan(
			&member.GuildID, &member.PlayerGUID, &member.Rank, &member.PublicNote, &member.OfficerNote, &member.Name,
			&member.Lvl, &member.Class, &member.Gender, &member.AreaID, &member.Account, &member.Status, &member.LogoutTime,
		); err != nil {
			return nil, err
		}
		guild.GuildMembers = append(guild.GuildMembers, &member)
	}
	if memberRows.Err() != nil {
		return nil, memberRows.Err()
	}

	return &guild, nil
}

// AddGuildInvite links user invite to a specific guild.
func (g *guildsMySQLRepo) AddGuildInvite(ctx context.Context, realmID uint32, charGUID, guildID uint64) error {
	_, err := g.db.PreparedStatement(realmID, StmtAddGuildInvite).ExecContext(ctx, charGUID, guildID)
	return err
}

// GuildIDByCharInvite returns guild id by invited character.
func (g *guildsMySQLRepo) GuildIDByCharInvite(ctx context.Context, realmID uint32, charGUID uint64) (uint64, error) {
	row := g.db.PreparedStatement(realmID, StmtGetGuildInvite).QueryRowContext(ctx, charGUID)
	if row.Err() != nil {
		return 0, row.Err()
	}

	var guildID uint64
	err := row.Scan(&guildID)
	if err != nil {
		return 0, err
	}
	return guildID, nil
}

// RemoveGuildInviteForCharacter removes guild invite by character.
func (g *guildsMySQLRepo) RemoveGuildInviteForCharacter(ctx context.Context, realmID uint32, charGUID uint64) error {
	_, err := g.db.PreparedStatement(realmID, StmtRemoveGuildInvite).ExecContext(ctx, charGUID)
	return err
}

// GuildIDByRealmAndMemberGUID returns guild id by guild member GUID.
func (g *guildsMySQLRepo) GuildIDByRealmAndMemberGUID(ctx context.Context, realmID uint32, memberGUID uint64) (uint64, error) {
	row := g.db.PreparedStatement(realmID, StmtGetGuildIDByMemberGUID).QueryRowContext(ctx, memberGUID)
	if row.Err() != nil {
		return 0, row.Err()
	}

	var guildID uint64
	err := row.Scan(&guildID)
	if err != nil {
		return 0, err
	}
	return guildID, nil
}

// AddGuildMember adds guild member to the guild.
func (g *guildsMySQLRepo) AddGuildMember(ctx context.Context, realmID uint32, member GuildMember) error {
	_, err := g.db.PreparedStatement(realmID, StmtAddGuildMember).ExecContext(
		ctx, member.GuildID, member.PlayerGUID, member.Rank, member.PublicNote, member.OfficerNote,
	)
	return err
}

// RemoveGuildMember removes guild member from the guild.
func (g *guildsMySQLRepo) RemoveGuildMember(ctx context.Context, realmID uint32, characterGUID uint64) error {
	_, err := g.db.PreparedStatement(realmID, StmtRemoveGuildMember).ExecContext(ctx, characterGUID)
	return err
}

// SetMessageOfTheDay updates message of the day for the guild.
func (g *guildsMySQLRepo) SetMessageOfTheDay(ctx context.Context, realmID uint32, guildID uint64, message string) error {
	_, err := g.db.PreparedStatement(realmID, StmtUpdateGuildMessageOfTheDay).ExecContext(ctx, message, guildID)
	return err
}

// SetMemberPublicNote sets public not for guild member.
func (g *guildsMySQLRepo) SetMemberPublicNote(ctx context.Context, realmID uint32, memberGUID uint64, note string) error {
	_, err := g.db.PreparedStatement(realmID, StmtUpdateGuildMemberPublicNote).ExecContext(ctx, note, memberGUID)
	return err
}

// SetMemberOfficerNote sets officer not for guild member.
func (g *guildsMySQLRepo) SetMemberOfficerNote(ctx context.Context, realmID uint32, memberGUID uint64, note string) error {
	_, err := g.db.PreparedStatement(realmID, StmtUpdateGuildMemberOfficersNote).ExecContext(ctx, note, memberGUID)
	return err
}

// SetMemberRank sets rank for the guild member.
func (g *guildsMySQLRepo) SetMemberRank(ctx context.Context, realmID uint32, memberGUID uint64, rank uint8) error {
	_, err := g.db.PreparedStatement(realmID, StmtUpdateGuildMemberRank).ExecContext(ctx, rank, memberGUID)
	return err
}

// SetGuildInfo updates guild info text of the guild.
func (g *guildsMySQLRepo) SetGuildInfo(ctx context.Context, realmID uint32, guildID uint64, info string) error {
	_, err := g.db.PreparedStatement(realmID, StmtUpdateGuildInfo).ExecContext(ctx, info, guildID)
	return err
}

// UpdateGuildRank updates guild rank.
func (g *guildsMySQLRepo) UpdateGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8, name string, rights, moneyPerDay uint32) error {
	_, err := g.db.PreparedStatement(realmID, StmtUpdateGuildRank).ExecContext(ctx, name, rights, moneyPerDay, rank, guildID)
	return err
}

// AddGuildRank adds guild rank.
func (g *guildsMySQLRepo) AddGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8, name string, rights, moneyPerDay uint32) error {
	_, err := g.db.PreparedStatement(realmID, StmtAddGuildRank).ExecContext(ctx, guildID, rank, name, rights, moneyPerDay)
	return err
}

// DeleteLowestGuildRank deletes lowes guild rank.
func (g *guildsMySQLRepo) DeleteLowestGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8) error {
	_, err := g.db.PreparedStatement(realmID, StmtDeleteGuildRank).ExecContext(ctx, guildID, rank)
	return err
}

// CreateGuild creates a guild with the given ranks and the leader as first member.
// The guild id is allocated as MAX(guildid)+1 inside the transaction; a concurrent
// in-process creation on a worldserver can win the same id, so the primary key
// conflict is retried with a fresh id.
func (g *guildsMySQLRepo) CreateGuild(ctx context.Context, realmID uint32, name string, leaderGUID uint64, ranks []GuildRank) (uint64, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		id, err := g.tryCreateGuild(ctx, realmID, name, leaderGUID, ranks)
		if err == nil || errors.Is(err, ErrGuildNameTaken) {
			return id, err
		}
		var mysqlErr *mysql.MySQLError
		if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 {
			return 0, err
		}
		lastErr = err
	}
	return 0, fmt.Errorf("guild id allocation kept conflicting, err: %w", lastErr)
}

func (g *guildsMySQLRepo) tryCreateGuild(ctx context.Context, realmID uint32, name string, leaderGUID uint64, ranks []GuildRank) (uint64, error) {
	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var existing uint64
	err = tx.QueryRowContext(ctx, "SELECT guildid FROM guild WHERE name = ?", name).Scan(&existing)
	if err == nil {
		return 0, ErrGuildNameTaken
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	var id uint64
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(guildid), 0) + 1 FROM guild").Scan(&id); err != nil {
		return 0, err
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO guild (guildid, name, leaderguid, info, motd, createdate, BankMoney) VALUES (?, ?, ?, '', '', ?, 0)",
		id, name, leaderGUID, time.Now().Unix(),
	)
	if err != nil {
		return 0, err
	}

	for i := range ranks {
		_, err = tx.ExecContext(ctx,
			"INSERT INTO guild_rank (guildid, rid, rname, rights, BankMoneyPerDay) VALUES (?, ?, ?, ?, ?)",
			id, ranks[i].Rank, ranks[i].Name, ranks[i].Rights, ranks[i].MoneyPerDay,
		)
		if err != nil {
			return 0, err
		}
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO guild_member (guildid, guid, `rank`, pnote, offnote) VALUES (?, ?, ?, '', '')",
		id, leaderGUID, uint8(GuildRankGuildMaster),
	)
	if err != nil {
		return 0, err
	}

	return id, tx.Commit()
}
