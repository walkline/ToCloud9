package repo

import "fmt"

const (
	// StmtAddGuildInvite to bind character to the guild invite.
	StmtAddGuildInvite CharsPreparedStatements = iota

	// StmtGetGuildInvite returns guild id by character guid.
	StmtGetGuildInvite

	// StmtRemoveGuildInvite removes character invite.
	StmtRemoveGuildInvite

	// StmtAddGuildMember adds new guild member.
	StmtAddGuildMember

	// StmtRemoveGuildMember deletes guild member.
	StmtRemoveGuildMember

	// StmtGetGuildIDByMemberGUID returns guild id by guild member guid.
	StmtGetGuildIDByMemberGUID

	// StmtUpdateGuildMessageOfTheDay updates message of the day for the guild.
	StmtUpdateGuildMessageOfTheDay

	// StmtUpdateGuildMemberPublicNote updates public note of the guild member.
	StmtUpdateGuildMemberPublicNote

	// StmtUpdateGuildMemberOfficersNote updates officers note of the guild member.
	StmtUpdateGuildMemberOfficersNote

	// StmtUpdateGuildMemberRank updates rank field for guild member.
	StmtUpdateGuildMemberRank

	// StmtUpdateGuildInfo updates guilds info message.
	StmtUpdateGuildInfo

	// StmtUpdateGuildRank updates guilds rank.
	StmtUpdateGuildRank

	// StmtAddGuildRank adds guild rank.
	StmtAddGuildRank

	// StmtDeleteGuildRank deletes guild rank.
	StmtDeleteGuildRank
)

// CharsPreparedStatements represents prepared statements for the characters database.
// Implements sharedrepo.PreparedStatement interface.
type CharsPreparedStatements uint32

// ID returns identifier of prepared statement.
func (s CharsPreparedStatements) ID() uint32 {
	return uint32(s)
}

// Stmt returns prepared statement as string.
func (s CharsPreparedStatements) Stmt() string {
	switch s {
	case StmtAddGuildInvite:
		return "REPLACE INTO guild_invites (charGuid, guildId) VALUES (?, ?)"
	case StmtGetGuildInvite:
		return "SELECT guildId FROM guild_invites WHERE charGuid = ?"
	case StmtRemoveGuildInvite:
		return "DELETE FROM guild_invites WHERE charGuid = ?"
	case StmtAddGuildMember:
		return "INSERT INTO guild_member (guildid, guid, `rank`, pnote, offnote) VALUES (?, ?, ?, ?, ?)"
	case StmtRemoveGuildMember:
		return "DELETE FROM guild_member WHERE guid = ?"
	case StmtGetGuildIDByMemberGUID:
		return "SELECT guildid FROM guild_member WHERE guid = ?"
	case StmtUpdateGuildMessageOfTheDay:
		return "UPDATE guild SET motd = ? WHERE guildid = ?"
	case StmtUpdateGuildMemberPublicNote:
		return "UPDATE guild_member SET pnote = ? WHERE guid = ?"
	case StmtUpdateGuildMemberOfficersNote:
		return "UPDATE guild_member SET offnote = ? WHERE guid = ?"
	case StmtUpdateGuildMemberRank:
		return "UPDATE guild_member SET `rank` = ? WHERE guid = ?"
	case StmtUpdateGuildInfo:
		return "UPDATE guild SET info = ? WHERE guildid = ?"
	case StmtUpdateGuildRank:
		return "UPDATE guild_rank SET rname = ?, rights = ?, BankMoneyPerDay = ? WHERE rid = ? AND guildid = ?"
	case StmtAddGuildRank:
		return "INSERT INTO guild_rank (guildid, rid, rname, rights, BankMoneyPerDay) VALUES (?, ?, ?, ?, ?)"
	case StmtDeleteGuildRank:
		return "DELETE FROM guild_rank WHERE guildid = ? AND rid >= ?"
	}
	panic(fmt.Errorf("unk stmt %d", s))
}
