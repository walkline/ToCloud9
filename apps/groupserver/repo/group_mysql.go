package repo

import (
	"context"
	"database/sql"
	"fmt"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

type groupsRepoMysql struct {
	db shrepo.CharactersDB
}

func NewMysqlGroupsRepo(db shrepo.CharactersDB) GroupsRepo {
	db.SetPreparedStatement(StmtReplaceGroupInvite)
	db.SetPreparedStatement(StmtSelectGroupInviteByInvited)
	db.SetPreparedStatement(StmtInsertNewGroup)
	db.SetPreparedStatement(StmtInsertNewGroupMember)
	db.SetPreparedStatement(StmtUpdateGroupWithID)
	db.SetPreparedStatement(StmtUpdateGroupMemberWithID)
	db.SetPreparedStatement(StmtDeleteGroupMembersWithGroupID)
	db.SetPreparedStatement(StmtDeleteGroupWithID)
	db.SetPreparedStatement(StmtDeleteGroupMemberWithID)

	return &groupsRepoMysql{
		db: db,
	}
}

func (g groupsRepoMysql) LoadAllForRealm(ctx context.Context, realmID uint32) (map[uint]*Group, error) {
	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx, `SELECT 
	guid, leaderGuid, lootMethod, looterGuid, lootThreshold, 
	icon1, icon2, icon3, icon4, icon5, icon6, icon7, icon8, 
	groupType, difficulty, raidDifficulty, masterLooterGuid
FROM `+"`groups`")
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
			&group.GroupType, &group.Difficulty, &group.RaidDifficulty, &group.MasterLooterGuid,
		)
		if err != nil {
			return nil, err
		}

		result[group.ID] = &group
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	rows.Close()

	rows, err = g.db.DBByRealm(realmID).QueryContext(ctx, `SELECT
	gm.guid, gm.memberGuid, gm.memberFlags, gm.subgroup, gm.roles, c.name, c.online
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

		group := result[gm.GroupID]
		if group != nil {
			group.Members = append(group.Members, gm)
		}
	}

	return result, nil
}

func (g groupsRepoMysql) Create(ctx context.Context, realmID uint32, group *Group) error {
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

	if len(group.Members) == 0 {
		return nil
	}

	for i, m := range group.Members {
		group.Members[i].GroupID = group.ID
		_, err = g.db.PreparedStatement(realmID, StmtInsertNewGroupMember).ExecContext(
			ctx, group.ID, m.MemberGUID, m.MemberFlags, m.SubGroup, m.Roles,
		)
		if err != nil {
			return fmt.Errorf("can't insert group member, err: %w", err)
		}
	}

	return nil
}

func (g groupsRepoMysql) GroupByID(ctx context.Context, realmID uint32, partyID uint, loadMembers bool) (*Group, error) {
	//TODO implement me
	panic("implement me")
}

func (g groupsRepoMysql) GroupIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint, error) {
	panic("implement me")
}

func (g groupsRepoMysql) AddMember(ctx context.Context, realmID uint32, m *GroupMember) error {
	_, err := g.db.PreparedStatement(realmID, StmtInsertNewGroupMember).ExecContext(
		ctx, m.GroupID, m.MemberGUID, m.MemberFlags, m.SubGroup, m.Roles,
	)
	return err
}

func (g groupsRepoMysql) Update(ctx context.Context, realmID uint32, group *Group) error {
	_, err := g.db.PreparedStatement(realmID, StmtUpdateGroupWithID).ExecContext(
		ctx, group.LeaderGUID, group.LootMethod, group.LooterGUID, group.LootThreshold,
		group.TargetIcons[0], group.TargetIcons[1], group.TargetIcons[2], group.TargetIcons[3],
		group.TargetIcons[4], group.TargetIcons[5], group.TargetIcons[6], group.TargetIcons[7],
		group.GroupType, group.Difficulty, group.RaidDifficulty, group.MasterLooterGuid, group.ID,
	)
	return err
}

func (g groupsRepoMysql) UpdateMember(ctx context.Context, realmID uint32, m *GroupMember) error {
	_, err := g.db.PreparedStatement(realmID, StmtUpdateGroupMemberWithID).ExecContext(
		ctx, m.GroupID, m.MemberFlags, m.SubGroup, m.Roles, m.MemberGUID,
	)
	return err
}

func (g groupsRepoMysql) RemoveMember(ctx context.Context, realmID uint32, memberGUID uint64) error {
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
	_, err = g.db.PreparedStatement(realmID, StmtDeleteGroupWithID).ExecContext(
		ctx, groupID,
	)
	return err
}

func (g groupsRepoMysql) AddInvite(ctx context.Context, realmID uint32, invite GroupInvite) error {
	_, err := g.db.PreparedStatement(realmID, StmtReplaceGroupInvite).ExecContext(
		ctx, invite.Invitee, invite.Inviter, invite.GroupID, invite.InviteeName, invite.InviterName,
	)
	return err
}

func (g groupsRepoMysql) GetInviteByInvitedPlayer(ctx context.Context, realmID uint32, invitedPlayer uint64) (*GroupInvite, error) {
	row := g.db.PreparedStatement(realmID, StmtSelectGroupInviteByInvited).QueryRowContext(ctx, invitedPlayer)

	groupInvite := GroupInvite{
		Invitee: invitedPlayer,
	}

	err := row.Scan(&groupInvite.Inviter, &groupInvite.GroupID, &groupInvite.InviteeName, &groupInvite.InviterName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &groupInvite, nil
}
