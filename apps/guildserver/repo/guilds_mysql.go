package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
	wowguid "github.com/walkline/ToCloud9/shared/wow/guid"
)

const characterSocialFlagIgnore = 0x02

type guildsMySQLRepo struct {
	db      shrepo.CharactersDB
	worldDB *sql.DB
}

func NewGuildsMySQLRepo(db shrepo.CharactersDB) (GuildsRepo, error) {
	return NewGuildsMySQLRepoWithWorldDB(db, nil)
}

func NewGuildsMySQLRepoWithWorldDB(db shrepo.CharactersDB, worldDB *sql.DB) (GuildsRepo, error) {
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
	db.SetPreparedStatement(StmtAddGuildPetitionSignature)

	return &guildsMySQLRepo{
		db:      db,
		worldDB: worldDB,
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

		guild.RealmID = realmID
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
	guildid, gm.guid, ` + "`rank`" + `, pnote, offnote,
	COALESCE(w.tab0, 0), COALESCE(w.tab1, 0), COALESCE(w.tab2, 0), COALESCE(w.tab3, 0),
	COALESCE(w.tab4, 0), COALESCE(w.tab5, 0), COALESCE(w.money, 0),
	c.name, c.level, c.class, c.gender, c.zone, c.account, c.online, c.logout_time
FROM guild_member gm
LEFT JOIN guild_member_withdraw w ON gm.guid = w.guid
LEFT JOIN characters c ON c.guid = gm.guid ORDER BY guildid ASC`)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		member := GuildMember{}
		err = rows.Scan(
			&member.GuildID, &member.PlayerGUID, &member.Rank, &member.PublicNote, &member.OfficerNote,
			&member.BankWithdraw[0], &member.BankWithdraw[1], &member.BankWithdraw[2], &member.BankWithdraw[3],
			&member.BankWithdraw[4], &member.BankWithdraw[5], &member.BankWithdraw[6],
			&member.Name, &member.Lvl, &member.Class, &member.Gender, &member.AreaID, &member.Account, &member.Status, &member.LogoutTime,
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

	if err = g.loadBankTabsForGuilds(ctx, realmID, result); err != nil {
		return nil, err
	}

	if err = g.loadBankRightsForGuilds(ctx, realmID, result); err != nil {
		return nil, err
	}

	return result, nil
}

// GuildByRealmAndID loads guild by realm and id.
func (g *guildsMySQLRepo) GuildByRealmAndID(ctx context.Context, realmID uint32, guildID uint64) (*Guild, error) {
	guild := Guild{RealmID: realmID}
	err := g.db.DBByRealm(realmID).QueryRowContext(ctx, `
SELECT
	g.guildid, g.name, g.leaderguid, g.EmblemStyle, g.EmblemColor, g.BorderStyle,
	g.BorderColor, g.BackgroundColor, g.info, g.motd, g.createdate, g.BankMoney
FROM guild g
WHERE g.guildid = ?`, guildID).Scan(
		&guild.ID, &guild.Name, &guild.LeaderGUID, &guild.Emblem.Style, &guild.Emblem.Color,
		&guild.Emblem.BorderStyle, &guild.Emblem.BorderColor, &guild.Emblem.BackgroundColor,
		&guild.Info, &guild.MessageOfTheDay, &guild.CrateTimeUnix, &guild.BankMoney,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx, `
SELECT guildid, rid, rname, rights, BankMoneyPerDay
FROM guild_rank
WHERE guildid = ?
ORDER BY rid ASC`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		rank := GuildRank{}
		err = rows.Scan(&rank.GuildID, &rank.Rank, &rank.Name, &rank.Rights, &rank.MoneyPerDay)
		if err != nil {
			return nil, err
		}

		guild.GuildRanks = append(guild.GuildRanks, rank)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if err = g.loadBankTabsForGuild(ctx, realmID, &guild); err != nil {
		return nil, err
	}

	if err = g.loadBankRightsForGuild(ctx, realmID, &guild); err != nil {
		return nil, err
	}

	rows, err = g.db.DBByRealm(realmID).QueryContext(ctx, `
SELECT
	guildid, gm.guid, `+"`rank`"+`, pnote, offnote,
	COALESCE(w.tab0, 0), COALESCE(w.tab1, 0), COALESCE(w.tab2, 0), COALESCE(w.tab3, 0),
	COALESCE(w.tab4, 0), COALESCE(w.tab5, 0), COALESCE(w.money, 0),
	c.name, c.level, c.class, c.gender, c.zone, c.account, c.online, c.logout_time
FROM guild_member gm
LEFT JOIN guild_member_withdraw w ON gm.guid = w.guid
LEFT JOIN characters c ON c.guid = gm.guid
WHERE gm.guildid = ?
ORDER BY gm.guid ASC`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		member := GuildMember{}
		err = rows.Scan(
			&member.GuildID, &member.PlayerGUID, &member.Rank, &member.PublicNote, &member.OfficerNote,
			&member.BankWithdraw[0], &member.BankWithdraw[1], &member.BankWithdraw[2], &member.BankWithdraw[3],
			&member.BankWithdraw[4], &member.BankWithdraw[5], &member.BankWithdraw[6],
			&member.Name, &member.Lvl, &member.Class, &member.Gender, &member.AreaID, &member.Account, &member.Status, &member.LogoutTime,
		)
		if err != nil {
			return nil, err
		}

		guild.GuildMembers = append(guild.GuildMembers, &member)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return &guild, nil
}

func (g *guildsMySQLRepo) loadBankTabsForGuilds(ctx context.Context, realmID uint32, guilds map[uint64]*Guild) error {
	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx, `
SELECT guildid, COUNT(*)
FROM guild_bank_tab
GROUP BY guildid`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var guildID uint64
		var tabs uint32
		if err = rows.Scan(&guildID, &tabs); err != nil {
			return err
		}

		if guild := guilds[guildID]; guild != nil {
			guild.PurchasedBankTabs = clampGuildBankTabCount(tabs)
		}
	}

	return rows.Err()
}

func (g *guildsMySQLRepo) loadBankTabsForGuild(ctx context.Context, realmID uint32, guild *Guild) error {
	var tabs uint32
	err := g.db.DBByRealm(realmID).QueryRowContext(ctx, `
SELECT COUNT(*)
FROM guild_bank_tab
WHERE guildid = ?`, guild.ID).Scan(&tabs)
	if err != nil {
		return err
	}

	guild.PurchasedBankTabs = clampGuildBankTabCount(tabs)
	return nil
}

func (g *guildsMySQLRepo) loadBankRightsForGuilds(ctx context.Context, realmID uint32, guilds map[uint64]*Guild) error {
	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx, `
SELECT guildid, TabId, rid, gbright, SlotPerDay
FROM guild_bank_right
ORDER BY guildid ASC, rid ASC, TabId ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var guildID uint64
		var tabID uint8
		var rankID uint8
		var flags uint32
		var withdrawItemLimit uint32
		if err = rows.Scan(&guildID, &tabID, &rankID, &flags, &withdrawItemLimit); err != nil {
			return err
		}

		applyGuildBankRight(guilds[guildID], tabID, rankID, flags, withdrawItemLimit)
	}

	return rows.Err()
}

func (g *guildsMySQLRepo) loadBankRightsForGuild(ctx context.Context, realmID uint32, guild *Guild) error {
	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx, `
SELECT TabId, rid, gbright, SlotPerDay
FROM guild_bank_right
WHERE guildid = ?
ORDER BY rid ASC, TabId ASC`, guild.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tabID uint8
		var rankID uint8
		var flags uint32
		var withdrawItemLimit uint32
		if err = rows.Scan(&tabID, &rankID, &flags, &withdrawItemLimit); err != nil {
			return err
		}

		applyGuildBankRight(guild, tabID, rankID, flags, withdrawItemLimit)
	}

	return rows.Err()
}

func applyGuildBankRight(guild *Guild, tabID, rankID uint8, flags, withdrawItemLimit uint32) {
	if guild == nil || tabID >= GuildBankMaxTabs {
		return
	}

	for i := range guild.GuildRanks {
		if guild.GuildRanks[i].Rank == rankID {
			guild.GuildRanks[i].BankTabRights[tabID] = GuildBankTabRight{
				TabID:             tabID,
				Flags:             flags,
				WithdrawItemLimit: withdrawItemLimit,
			}
			return
		}
	}
}

func clampGuildBankTabCount(tabs uint32) uint8 {
	if tabs > GuildBankMaxTabs {
		return GuildBankMaxTabs
	}

	return uint8(tabs)
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
	if err == sql.ErrNoRows {
		return 0, nil
	}
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
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return guildID, nil
}

func (g *guildsMySQLRepo) IgnoredByGuildMembers(ctx context.Context, realmID uint32, senderGUID uint64, receiverGUIDs []uint64) (map[uint64]bool, error) {
	ignored := make(map[uint64]bool)
	if len(receiverGUIDs) == 0 {
		return ignored, nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(receiverGUIDs)), ",")
	args := make([]any, 0, len(receiverGUIDs)+2)
	for _, receiverGUID := range receiverGUIDs {
		args = append(args, receiverGUID)
	}
	args = append(args, senderGUID, characterSocialFlagIgnore)

	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx, fmt.Sprintf(
		"SELECT guid FROM character_social WHERE guid IN (%s) AND friend = ? AND (flags & ?) <> 0",
		placeholders,
	), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var receiverGUID uint64
		if err := rows.Scan(&receiverGUID); err != nil {
			return nil, err
		}
		ignored[receiverGUID] = true
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return ignored, nil
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
func (g *guildsMySQLRepo) UpdateGuildRank(ctx context.Context, realmID uint32, guildID uint64, rank uint8, name string, rights, moneyPerDay uint32, bankTabRights [GuildBankMaxTabs]GuildBankTabRight) error {
	tx, err := g.db.DBByRealm(realmID).BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, StmtUpdateGuildRank.Stmt(), name, rights, moneyPerDay, rank, guildID); err != nil {
		tx.Rollback()
		return err
	}

	for _, right := range bankTabRights {
		if _, err = tx.ExecContext(ctx, `
INSERT INTO guild_bank_right (guildid, TabId, rid, gbright, SlotPerDay)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE gbright = VALUES(gbright), SlotPerDay = VALUES(SlotPerDay)`,
			guildID, right.TabID, rank, right.Flags, right.WithdrawItemLimit); err != nil {
			tx.Rollback()
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		tx.Rollback()
		return err
	}

	return nil
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

// GuildPetitionByGUID loads a native petition by item GUID.
func (g *guildsMySQLRepo) GuildPetitionByGUID(ctx context.Context, realmID uint32, petitionGUID uint64) (*GuildPetition, error) {
	petitionGuidLow := uint64(wowguid.New(petitionGUID).GetCounter())
	petition := GuildPetition{RealmID: realmID}
	err := g.db.DBByRealm(realmID).QueryRowContext(ctx, `
SELECT petition_id, ownerguid, petitionguid, name, type
FROM petition
WHERE petitionguid = ?`, petitionGuidLow).Scan(
		&petition.PetitionID,
		&petition.OwnerGUID,
		&petitionGuidLow,
		&petition.Name,
		&petition.Type,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	petition.PetitionGUID = wowguid.NewFromCounter(wowguid.Item, wowguid.LowType(petitionGuidLow)).GetRawValue()

	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx, `
SELECT playerguid, player_account
FROM petition_sign
WHERE petition_id = ?
ORDER BY playerguid ASC`, petition.PetitionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var signature GuildPetitionSignature
		if err = rows.Scan(&signature.PlayerGUID, &signature.PlayerAccount); err != nil {
			return nil, err
		}
		petition.Signatures = append(petition.Signatures, signature)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return &petition, nil
}

// AddGuildPetitionSignature persists a native guild petition signature.
func (g *guildsMySQLRepo) AddGuildPetitionSignature(ctx context.Context, realmID uint32, petitionID uint32, petitionGUID, ownerGUID, playerGUID uint64, playerAccount uint32) error {
	petitionGuidLow := uint64(wowguid.New(petitionGUID).GetCounter())
	_, err := g.db.PreparedStatement(realmID, StmtAddGuildPetitionSignature).ExecContext(ctx, ownerGUID, petitionGuidLow, petitionID, playerGUID, playerAccount)
	return err
}
