package repo

import (
	"context"
	"fmt"
)

type groupsRepoMysqlCmangos struct {
	groupsRepoMysql
}

func (g groupsRepoMysqlCmangos) LoadAllForRealm(ctx context.Context, realmID uint32) (map[uint]*Group, error) {
	rows, err := g.db.DBByRealm(realmID).QueryContext(ctx, `SELECT 
	groupId, leaderGuid, lootMethod, looterGuid, lootThreshold, 
	icon1, icon2, icon3, icon4, icon5, icon6, icon7, icon8, 
	groupType, difficulty, raiddifficulty
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
			&group.GroupType, &group.Difficulty, &group.RaidDifficulty,
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
	gm.groupId, gm.memberGuid, gm.subgroup, gm.assistant, c.name, c.online
FROM group_member gm
LEFT JOIN characters c ON c.guid = gm.memberGuid`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		gm := GroupMember{}
		err = rows.Scan(
			&gm.GroupID, &gm.MemberGUID, &gm.SubGroup, &gm.Roles, &gm.MemberName, &gm.IsOnline,
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

func (g groupsRepoMysqlCmangos) Create(ctx context.Context, realmID uint32, group *Group) error {
	execRes, err := g.db.PreparedStatement(realmID, StmtInsertNewGroup).ExecContext(
		ctx, group.LeaderGUID, group.LootMethod, group.LooterGUID, group.LootThreshold,
		group.TargetIcons[0], group.TargetIcons[1], group.TargetIcons[2], group.TargetIcons[3],
		group.TargetIcons[4], group.TargetIcons[5], group.TargetIcons[6], group.TargetIcons[7],
		group.GroupType, group.Difficulty, group.RaidDifficulty,
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
			ctx, group.ID, m.MemberGUID, m.SubGroup, m.IsAssistant(),
		)
		if err != nil {
			return fmt.Errorf("can't insert group member, err: %w", err)
		}
	}

	return nil
}

func (g groupsRepoMysqlCmangos) AddMember(ctx context.Context, realmID uint32, m *GroupMember) error {
	_, err := g.db.PreparedStatement(realmID, StmtInsertNewGroupMember).ExecContext(
		ctx, m.GroupID, m.MemberGUID, m.SubGroup, m.IsAssistant(),
	)
	return err
}

func (g groupsRepoMysqlCmangos) Update(ctx context.Context, realmID uint32, group *Group) error {
	_, err := g.db.PreparedStatement(realmID, StmtUpdateGroupWithID).ExecContext(
		ctx, group.LeaderGUID, group.LootMethod, group.LooterGUID, group.LootThreshold,
		group.TargetIcons[0], group.TargetIcons[1], group.TargetIcons[2], group.TargetIcons[3],
		group.TargetIcons[4], group.TargetIcons[5], group.TargetIcons[6], group.TargetIcons[7],
		group.GroupType, group.Difficulty, group.RaidDifficulty, group.ID,
	)
	return err
}

func (g groupsRepoMysqlCmangos) UpdateMember(ctx context.Context, realmID uint32, m *GroupMember) error {
	_, err := g.db.PreparedStatement(realmID, StmtUpdateGroupMemberWithID).ExecContext(
		ctx, m.GroupID, m.SubGroup, m.IsAssistant(), m.MemberGUID,
	)
	return err
}
