package repo

import (
	"context"
	"database/sql"
	"fmt"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

type groupsRepoMysql struct {
	db shrepo.CharactersDB
}

func NewMysqlGroupsRepo(db shrepo.CharactersDB) GroupsRepo {
	db.SetPreparedStatement(StmtReplaceGroupInvite)
	db.SetPreparedStatement(StmtSelectGroupInviteByInvited)
	db.SetPreparedStatement(StmtDeleteGroupInviteByInvited)
	db.SetPreparedStatement(StmtInsertNewGroup)
	db.SetPreparedStatement(StmtInsertNewGroupMember)
	db.SetPreparedStatement(StmtUpdateGroupWithID)
	db.SetPreparedStatement(StmtUpdateGroupMemberWithID)
	db.SetPreparedStatement(StmtDeleteGroupMembersWithGroupID)
	db.SetPreparedStatement(StmtDeleteGroupWithID)
	db.SetPreparedStatement(StmtDeleteGroupMemberWithID)
	db.SetPreparedStatement(StmtReplaceLfgData)
	db.SetPreparedStatement(StmtDeleteLfgData)

	return &groupsRepoMysql{
		db: db,
	}
}

func (g groupsRepoMysql) LoadAllForRealm(ctx context.Context, realmID uint32) (map[uint]*Group, error) {
	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx, `SELECT
		g.guid, g.leaderGuid, g.lootMethod, g.looterGuid, g.lootThreshold,
		g.icon1, g.icon2, g.icon3, g.icon4, g.icon5, g.icon6, g.icon7, g.icon8,
		g.groupType, g.difficulty, g.raidDifficulty, g.masterLooterGuid, COALESCE(ld.dungeon, 0)
	FROM `+"`groups`"+` g
	LEFT JOIN lfg_data ld ON ld.guid = g.guid`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	result := map[uint]*Group{}
	for rows.Next() {
		group := Group{}
		err = rows.Scan(
			&group.ID, &group.LeaderGUID, &group.LootMethod, &group.LooterGUID, &group.LootThreshold,
			&group.TargetIcons[0], &group.TargetIcons[1], &group.TargetIcons[2], &group.TargetIcons[3],
			&group.TargetIcons[4], &group.TargetIcons[5], &group.TargetIcons[6], &group.TargetIcons[7],
			&group.GroupType, &group.Difficulty, &group.RaidDifficulty, &group.MasterLooterGuid, &group.LfgDungeonEntry,
		)
		if err != nil {
			return nil, err
		}
		group.RealmID = realmID
		group.LeaderGUID = guid.NormalizePlayerGUIDForRealm(realmID, group.LeaderGUID)
		group.LooterGUID = guid.NormalizePlayerGUIDForRealm(realmID, group.LooterGUID)
		group.MasterLooterGuid = guid.NormalizePlayerGUIDForRealm(realmID, group.MasterLooterGuid)

		result[group.ID] = &group
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	rows.Close()

	rows, err = g.db.DBByRealm(realmID).QueryContext(ctx, `SELECT
	gm.guid, gm.memberGuid, gm.memberFlags, gm.subgroup, gm.roles, COALESCE(NULLIF(gm.memberName, ''), c.name, ''), COALESCE(c.online, 0)
FROM group_member gm
LEFT JOIN characters c ON c.guid = gm.memberGuid`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		gm := GroupMember{}
		err = rows.Scan(
			&gm.GroupID, &gm.MemberGUID, &gm.MemberFlags,
			&gm.SubGroup, &gm.Roles, &gm.MemberName, &gm.IsOnline,
		)
		if err != nil {
			return nil, err
		}
		gm.RealmID = guid.PlayerRealmIDOrDefault(realmID, gm.MemberGUID)
		gm.MemberGUID = guid.PlayerGUIDForRealm(realmID, gm.RealmID, gm.MemberGUID)

		group := result[gm.GroupID]
		if group != nil {
			group.Members = append(group.Members, gm)
		}
	}

	return result, nil
}

func (g groupsRepoMysql) Create(ctx context.Context, realmID uint32, group *Group) error {
	group.RealmID = realmID
	execRes, err := g.db.PreparedStatement(realmID, StmtInsertNewGroup).ExecContext(
		ctx, group.LeaderGUID, group.LootMethod, group.LooterGUID, group.LootThreshold,
		group.TargetIcons[0], group.TargetIcons[1], group.TargetIcons[2], group.TargetIcons[3],
		group.TargetIcons[4], group.TargetIcons[5], group.TargetIcons[6], group.TargetIcons[7],
		group.GroupType, group.Difficulty, group.RaidDifficulty, group.MasterLooterGuid,
	)
	if err != nil {
		return err
	}

	id, err := execRes.LastInsertId()
	if err != nil {
		return err
	}

	group.ID = uint(id)

	if err := g.syncLfgData(ctx, realmID, group); err != nil {
		return err
	}

	if len(group.Members) == 0 {
		return nil
	}

	for i, m := range group.Members {
		group.Members[i].GroupID = group.ID
		group.Members[i].RealmID = guid.PlayerRealmIDOrDefault(realmID, m.MemberGUID)
		group.Members[i].MemberGUID = guid.PlayerGUIDForRealm(realmID, group.Members[i].RealmID, m.MemberGUID)
		_, err = g.db.PreparedStatement(realmID, StmtInsertNewGroupMember).ExecContext(
			ctx, group.ID, group.Members[i].MemberGUID, group.Members[i].MemberName, group.Members[i].MemberFlags, group.Members[i].SubGroup, group.Members[i].Roles,
		)
		if err != nil {
			return fmt.Errorf("can't insert group member, err: %w", err)
		}
	}

	return nil
}

func (g groupsRepoMysql) GroupByID(ctx context.Context, realmID uint32, partyID uint, loadMembers bool) (*Group, error) {
	group := Group{}
	row := g.db.DBByRealm(realmID).QueryRowContext(ctx, `SELECT
		g.guid, g.leaderGuid, g.lootMethod, g.looterGuid, g.lootThreshold,
		g.icon1, g.icon2, g.icon3, g.icon4, g.icon5, g.icon6, g.icon7, g.icon8,
		g.groupType, g.difficulty, g.raidDifficulty, g.masterLooterGuid, COALESCE(ld.dungeon, 0)
	FROM `+"`groups`"+` g
	LEFT JOIN lfg_data ld ON ld.guid = g.guid
	WHERE g.guid = ?`, partyID)

	err := row.Scan(
		&group.ID, &group.LeaderGUID, &group.LootMethod, &group.LooterGUID, &group.LootThreshold,
		&group.TargetIcons[0], &group.TargetIcons[1], &group.TargetIcons[2], &group.TargetIcons[3],
		&group.TargetIcons[4], &group.TargetIcons[5], &group.TargetIcons[6], &group.TargetIcons[7],
		&group.GroupType, &group.Difficulty, &group.RaidDifficulty, &group.MasterLooterGuid, &group.LfgDungeonEntry,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	group.RealmID = realmID
	group.LeaderGUID = guid.NormalizePlayerGUIDForRealm(realmID, group.LeaderGUID)
	group.LooterGUID = guid.NormalizePlayerGUIDForRealm(realmID, group.LooterGUID)
	group.MasterLooterGuid = guid.NormalizePlayerGUIDForRealm(realmID, group.MasterLooterGuid)

	if !loadMembers {
		return &group, nil
	}

	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx, `SELECT
	gm.guid, gm.memberGuid, gm.memberFlags, gm.subgroup, gm.roles, COALESCE(NULLIF(gm.memberName, ''), c.name, ''), COALESCE(c.online, 0)
FROM group_member gm
LEFT JOIN characters c ON c.guid = gm.memberGuid
WHERE gm.guid = ?
ORDER BY gm.memberGuid ASC`, partyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		gm := GroupMember{}
		err = rows.Scan(
			&gm.GroupID, &gm.MemberGUID, &gm.MemberFlags,
			&gm.SubGroup, &gm.Roles, &gm.MemberName, &gm.IsOnline,
		)
		if err != nil {
			return nil, err
		}
		gm.RealmID = guid.PlayerRealmIDOrDefault(realmID, gm.MemberGUID)
		gm.MemberGUID = guid.PlayerGUIDForRealm(realmID, gm.RealmID, gm.MemberGUID)

		group.Members = append(group.Members, gm)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return &group, nil
}

func (g groupsRepoMysql) GroupIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint, error) {
	player = guid.PlayerGUIDForRealm(realmID, guid.PlayerRealmIDOrDefault(realmID, player), player)
	var groupID uint
	err := g.db.DBByRealm(realmID).QueryRowContext(ctx, `
SELECT guid
FROM group_member
WHERE memberGuid = ?
LIMIT 1`, player).Scan(&groupID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	return groupID, nil
}

func (g groupsRepoMysql) AddMember(ctx context.Context, realmID uint32, m *GroupMember) error {
	m.RealmID = guid.PlayerRealmIDOrDefault(realmID, m.MemberGUID)
	m.MemberGUID = guid.PlayerGUIDForRealm(realmID, m.RealmID, m.MemberGUID)
	_, err := g.db.PreparedStatement(realmID, StmtInsertNewGroupMember).ExecContext(
		ctx, m.GroupID, m.MemberGUID, m.MemberName, m.MemberFlags, m.SubGroup, m.Roles,
	)
	return err
}

func (g groupsRepoMysql) Update(ctx context.Context, realmID uint32, group *Group) error {
	group.RealmID = realmID
	_, err := g.db.PreparedStatement(realmID, StmtUpdateGroupWithID).ExecContext(
		ctx, group.LeaderGUID, group.LootMethod, group.LooterGUID, group.LootThreshold,
		group.TargetIcons[0], group.TargetIcons[1], group.TargetIcons[2], group.TargetIcons[3],
		group.TargetIcons[4], group.TargetIcons[5], group.TargetIcons[6], group.TargetIcons[7],
		group.GroupType, group.Difficulty, group.RaidDifficulty, group.MasterLooterGuid, group.ID,
	)
	if err != nil {
		return err
	}
	return g.syncLfgData(ctx, realmID, group)
}

func (g groupsRepoMysql) RegisterAcceptedLfgGroup(ctx context.Context, realmID uint32, group *Group) error {
	if group == nil {
		return nil
	}

	members := group.Members
	if group.ID == 0 {
		group.Members = nil
		if err := g.Create(ctx, realmID, group); err != nil {
			group.Members = members
			return err
		}
		group.Members = members
	} else {
		if err := g.Update(ctx, realmID, group); err != nil {
			return err
		}

		if _, err := g.db.PreparedStatement(realmID, StmtDeleteGroupMembersWithGroupID).ExecContext(ctx, group.ID); err != nil {
			return err
		}
	}

	for i, member := range members {
		group.Members[i].GroupID = group.ID
		group.Members[i].RealmID = guid.PlayerRealmIDOrDefault(realmID, member.MemberGUID)
		group.Members[i].MemberGUID = guid.PlayerGUIDForRealm(realmID, group.Members[i].RealmID, member.MemberGUID)
		if _, err := g.db.PreparedStatement(realmID, StmtDeleteGroupMemberWithID).ExecContext(ctx, group.Members[i].MemberGUID); err != nil {
			return err
		}
		if _, err := g.db.PreparedStatement(realmID, StmtInsertNewGroupMember).ExecContext(
			ctx,
			group.ID,
			group.Members[i].MemberGUID,
			group.Members[i].MemberName,
			group.Members[i].MemberFlags,
			group.Members[i].SubGroup,
			group.Members[i].Roles,
		); err != nil {
			return fmt.Errorf("can't insert accepted LFG group member, err: %w", err)
		}
	}

	return nil
}

func (g groupsRepoMysql) UpdateMember(ctx context.Context, realmID uint32, m *GroupMember) error {
	m.RealmID = guid.PlayerRealmIDOrDefault(realmID, m.MemberGUID)
	m.MemberGUID = guid.PlayerGUIDForRealm(realmID, m.RealmID, m.MemberGUID)
	_, err := g.db.PreparedStatement(realmID, StmtUpdateGroupMemberWithID).ExecContext(
		ctx, m.GroupID, m.MemberName, m.MemberFlags, m.SubGroup, m.Roles, m.MemberGUID,
	)
	return err
}

func (g groupsRepoMysql) RemoveMember(ctx context.Context, realmID uint32, memberGUID uint64) error {
	memberGUID = guid.PlayerGUIDForRealm(realmID, guid.PlayerRealmIDOrDefault(realmID, memberGUID), memberGUID)
	_, err := g.db.PreparedStatement(realmID, StmtDeleteGroupMemberWithID).ExecContext(
		ctx, memberGUID,
	)
	return err
}

func (g groupsRepoMysql) Delete(ctx context.Context, realmID uint32, groupID uint) error {
	_, err := g.db.PreparedStatement(realmID, StmtDeleteGroupMembersWithGroupID).ExecContext(
		ctx, groupID,
	)
	if err != nil {
		return err
	}
	if _, err = g.db.PreparedStatement(realmID, StmtDeleteLfgData).ExecContext(ctx, groupID); err != nil {
		return err
	}
	_, err = g.db.PreparedStatement(realmID, StmtDeleteGroupWithID).ExecContext(
		ctx, groupID,
	)
	return err
}

func (g groupsRepoMysql) syncLfgData(ctx context.Context, realmID uint32, group *Group) error {
	if group == nil || group.ID == 0 || group.LfgDungeonEntry == 0 || group.GroupType&GroupTypeFlagsLFG == 0 {
		return nil
	}

	const lfgStateDungeon uint8 = 5 // Mirrors AzerothCore lfg::LFG_STATE_DUNGEON.
	_, err := g.db.PreparedStatement(realmID, StmtReplaceLfgData).ExecContext(ctx, group.ID, group.LfgDungeonEntry, lfgStateDungeon)
	return err
}

func (g groupsRepoMysql) AddInvite(ctx context.Context, realmID uint32, invite GroupInvite) error {
	if invite.InviterRealmID == 0 {
		invite.InviterRealmID = guid.PlayerRealmIDOrDefault(realmID, invite.Inviter)
	}
	invite.Inviter = guid.PlayerGUIDForRealm(realmID, invite.InviterRealmID, invite.Inviter)
	if invite.InviteeRealmID == 0 {
		invite.InviteeRealmID = guid.PlayerRealmIDOrDefault(realmID, invite.Invitee)
	}
	invite.Invitee = guid.PlayerGUIDForRealm(realmID, invite.InviteeRealmID, invite.Invitee)
	if invite.GroupRealmID == 0 {
		invite.GroupRealmID = realmID
	}
	_, err := g.db.PreparedStatement(realmID, StmtReplaceGroupInvite).ExecContext(
		ctx, invite.Invitee, invite.Inviter, invite.GroupID, invite.InviteeName, invite.InviterName, invite.GroupRealmID,
	)
	return err
}

func (g groupsRepoMysql) GetInviteByInvitedPlayer(ctx context.Context, realmID uint32, invitedPlayer uint64) (*GroupInvite, error) {
	invitedPlayer = guid.PlayerGUIDForRealm(realmID, guid.PlayerRealmIDOrDefault(realmID, invitedPlayer), invitedPlayer)
	row := g.db.PreparedStatement(realmID, StmtSelectGroupInviteByInvited).QueryRowContext(ctx, invitedPlayer)

	groupInvite := GroupInvite{
		Invitee:        invitedPlayer,
		InviteeRealmID: guid.PlayerRealmIDOrDefault(realmID, invitedPlayer),
	}

	err := row.Scan(&groupInvite.Inviter, &groupInvite.GroupID, &groupInvite.InviteeName, &groupInvite.InviterName, &groupInvite.GroupRealmID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	groupInvite.InviterRealmID = guid.PlayerRealmIDOrDefault(realmID, groupInvite.Inviter)
	if groupInvite.GroupRealmID == 0 {
		groupInvite.GroupRealmID = realmID
	}

	return &groupInvite, nil
}

func (g groupsRepoMysql) RemoveInvite(ctx context.Context, realmID uint32, invitedPlayer uint64) error {
	invitedPlayer = guid.PlayerGUIDForRealm(realmID, guid.PlayerRealmIDOrDefault(realmID, invitedPlayer), invitedPlayer)
	_, err := g.db.PreparedStatement(realmID, StmtDeleteGroupInviteByInvited).ExecContext(ctx, invitedPlayer)
	return err
}
