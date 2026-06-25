package repo

import "fmt"

// CharsPreparedStatements represents prepared statements for the characters database.
// Implements sharedrepo.PreparedStatement interface.
type CharsPreparedStatements uint32

const (
	// StmtReplaceGroupInvite creates or replaces group invite.
	StmtReplaceGroupInvite CharsPreparedStatements = iota + 1

	// StmtSelectGroupInviteByInvited selects group invite by invited GUID.
	StmtSelectGroupInviteByInvited

	// StmtDeleteGroupInviteByInvited deletes group invite by invited GUID.
	StmtDeleteGroupInviteByInvited

	// StmtInsertNewGroup inserts new group record.
	StmtInsertNewGroup

	// StmtUpdateGroupWithID updates group with given ID.
	StmtUpdateGroupWithID

	// StmtInsertNewGroupMember inserts new group member record.
	StmtInsertNewGroupMember

	// StmtUpdateGroupMemberWithID updates group member with given ID.
	StmtUpdateGroupMemberWithID

	// StmtDeleteGroupMemberWithID deletes group member by member ID.
	StmtDeleteGroupMemberWithID

	// StmtDeleteGroupWithID deletes group with given ID.
	StmtDeleteGroupWithID

	// StmtDeleteGroupMembersWithGroupID deletes group members with given guild ID.
	StmtDeleteGroupMembersWithGroupID

	// StmtReplaceLfgData creates or replaces native LFG data for a group.
	StmtReplaceLfgData

	// StmtDeleteLfgData deletes native LFG data for a group.
	StmtDeleteLfgData
)

// ID returns identifier of prepared statement.
func (s CharsPreparedStatements) ID() uint32 {
	return uint32(s)
}

// Stmt returns prepared statement as string.
func (s CharsPreparedStatements) Stmt() string {
	switch s {
	case StmtReplaceGroupInvite:
		return "REPLACE INTO group_invites (invited, inviter, groupId, invitedName, inviterName, groupRealmId) VALUES (?, ?, ?, ?, ?, ?)"
	case StmtSelectGroupInviteByInvited:
		return "SELECT inviter, groupId, invitedName, inviterName, groupRealmId FROM group_invites WHERE invited = ?"
	case StmtDeleteGroupInviteByInvited:
		return "DELETE FROM group_invites WHERE invited = ?"
	case StmtInsertNewGroup:
		return `INSERT INTO 
    				` + "`groups`" + `(leaderGuid, lootMethod, looterGuid, lootThreshold, icon1, icon2, icon3, icon4, icon5, icon6, icon7, icon8, groupType, difficulty, raidDifficulty, masterLooterGuid) 
				VALUES 
				    (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	case StmtUpdateGroupWithID:
		return "UPDATE `groups` " + `
				SET 
                  leaderGuid = ?, lootMethod = ?, looterGuid = ?, lootThreshold = ?, 
                  icon1 = ?, icon2 = ?, icon3 = ?, icon4 = ?, icon5 = ?, icon6 = ?, icon7 = ?, icon8 = ?, 
                  groupType = ?, difficulty = ?, raidDifficulty = ?, masterLooterGuid = ? 
                WHERE guid = ?`
	case StmtInsertNewGroupMember:
		return `INSERT INTO group_member(guid, memberGuid, memberName, memberFlags, subgroup, roles)
				VALUES (?, ?, ?, ?, ?, ?)`
	case StmtUpdateGroupMemberWithID:
		return `UPDATE group_member 
				SET guid = ?, memberName = ?, memberFlags = ?, subgroup = ?, roles = ?
				WHERE memberGuid = ?`
	case StmtDeleteGroupMemberWithID:
		return "DELETE FROM group_member WHERE memberGuid = ?"
	case StmtDeleteGroupWithID:
		return "DELETE FROM `groups` WHERE guid = ?"
	case StmtDeleteGroupMembersWithGroupID:
		return "DELETE FROM group_member WHERE guid = ?"
	case StmtReplaceLfgData:
		return "REPLACE INTO lfg_data (guid, dungeon, state) VALUES (?, ?, ?)"
	case StmtDeleteLfgData:
		return "DELETE FROM lfg_data WHERE guid = ?"
	}
	panic(fmt.Errorf("unk stmt %d", s))
}
