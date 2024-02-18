package repo

import "fmt"

// tcAcScheme returns stmt for cmangos.
func (s CharsPreparedStatements) cmangosScheme() string {
	switch s {
	case StmtReplaceGroupInvite:
		return "REPLACE INTO group_invites (invited, inviter, groupId, invitedName, inviterName) VALUES (?, ?, ?, ?, ?)"
	case StmtSelectGroupInviteByInvited:
		return "SELECT inviter, groupId, invitedName, inviterName FROM group_invites WHERE invited = ?"
	case StmtInsertNewGroup:
		return `INSERT INTO 
    				` + "`groups`" + `(leaderGuid, mainTank, mainAssistant, lootMethod, looterGuid, lootThreshold, icon1, icon2, icon3, icon4, icon5, icon6, icon7, icon8, groupType, difficulty, raiddifficulty) 
				VALUES 
				    (?, 0, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	case StmtUpdateGroupWithID:
		return "UPDATE `groups` " + `
				SET 
                  leaderGuid = ?, lootMethod = ?, looterGuid = ?, lootThreshold = ?, 
                  icon1 = ?, icon2 = ?, icon3 = ?, icon4 = ?, icon5 = ?, icon6 = ?, icon7 = ?, icon8 = ?, 
                  groupType = ?, difficulty = ?, raidDifficulty = ? 
                WHERE groupId = ?`
	case StmtInsertNewGroupMember:
		return `INSERT INTO group_member(groupId, memberGuid, subgroup, assistant)
				VALUES (?, ?, ?, ?)`
	case StmtUpdateGroupMemberWithID:
		return `UPDATE group_member 
				SET groupId = ?, subgroup = ?, assistant = ?
				WHERE memberGuid = ?`
	case StmtDeleteGroupMemberWithID:
		return "DELETE FROM group_member WHERE memberGuid = ?"
	case StmtDeleteGroupWithID:
		return "DELETE FROM `groups` WHERE groupId = ?"
	case StmtDeleteGroupMembersWithGroupID:
		return "DELETE FROM group_member WHERE groupId = ?"
	}
	panic(fmt.Errorf("unk stmt %d", s))
}
