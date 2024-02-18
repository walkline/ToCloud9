package repo

import "fmt"

// tcAcScheme returns stmt for TrinityCore and AzerothCore.
func (s CharsPreparedStatements) tcAcScheme() string {
	switch s {
	case StmtReplaceGroupInvite:
		return "REPLACE INTO group_invites (invited, inviter, groupId, invitedName, inviterName) VALUES (?, ?, ?, ?, ?)"
	case StmtSelectGroupInviteByInvited:
		return "SELECT inviter, groupId, invitedName, inviterName FROM group_invites WHERE invited = ?"
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
		return `INSERT INTO group_member(guid, memberGuid, memberFlags, subgroup, roles)
				VALUES (?, ?, ?, ?, ?)`
	case StmtUpdateGroupMemberWithID:
		return `UPDATE group_member 
				SET guid = ?, memberFlags = ?, subgroup = ?, roles = ?
				WHERE memberGuid = ?`
	case StmtDeleteGroupMemberWithID:
		return "DELETE FROM group_member WHERE memberGuid = ?"
	case StmtDeleteGroupWithID:
		return "DELETE FROM `groups` WHERE guid = ?"
	case StmtDeleteGroupMembersWithGroupID:
		return "DELETE FROM group_member WHERE guid = ?"
	}
	panic(fmt.Errorf("unk stmt %d", s))
}
