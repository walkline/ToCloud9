package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

type CharactersPreparedStatements uint32

func (s CharactersPreparedStatements) Stmt() string {
	switch s {
	case StmtListCharactersToLogin:
		return `SELECT c.guid, c.account, c.name, c.race, c.class, c.gender, c.skin, c.face, c.hairStyle, c.hairColor, c.facialStyle, c.level, c.zone, c.map, c.position_x, c.position_y, c.position_z,
		IFNULL(gm.guildid, 0), c.playerFlags, c.extra_flags, c.at_login, IFNULL(cp.entry, 0), IFNULL(cp.modelid, 0), IFNULL(cp.level, 0), c.equipmentCache, IFNULL(cb.guid, 0)
		FROM characters AS c LEFT JOIN character_pet AS cp ON c.guid = cp.owner AND cp.slot = ? LEFT JOIN guild_member AS gm ON c.guid = gm.guid
		LEFT JOIN character_banned AS cb ON c.guid = cb.guid AND cb.active = 1 WHERE c.account = ? AND c.deleteInfos_Name IS NULL ORDER BY COALESCE(c.order, c.guid)`
	case StmtCharacterToLogin:
		return `SELECT c.guid, c.account, c.name, c.race, c.class, c.gender, c.skin, c.face, c.hairStyle, c.hairColor, c.facialStyle, c.level, c.zone, c.map, c.position_x, c.position_y, c.position_z,
		IFNULL(gm.guildid, 0), c.playerFlags, c.extra_flags, c.at_login, IFNULL(cp.entry, 0), IFNULL(cp.modelid, 0), IFNULL(cp.level, 0), c.equipmentCache, IFNULL(cb.guid, 0)
		FROM characters AS c LEFT JOIN character_pet AS cp ON c.guid = cp.owner AND cp.slot = ? LEFT JOIN guild_member AS gm ON c.guid = gm.guid
		LEFT JOIN character_banned AS cb ON c.guid = cb.guid AND cb.active = 1 WHERE c.guid = ? AND c.account = ? AND c.deleteInfos_Name IS NULL`
	case StmtSelectAccountData:
		return "SELECT type, time, data FROM account_data WHERE accountId = ?"
	case StmtReplaceAccountData:
		return "REPLACE INTO account_data (accountId, type, time, data) VALUES (?, ?, ?, ?)"
	case StmtSelectCharacterWithName:
		return "SELECT c.guid, account, race, class, gender, level, zone, map, position_x, position_y, position_z, IFNULL(gm.guildid, 0) FROM characters AS c LEFT JOIN guild_member AS gm ON c.guid = gm.guid WHERE name = ?"
	case StmtSelectCharacterWithGUID:
		return "SELECT c.guid, c.name, account, race, class, gender, level, zone, map, position_x, position_y, position_z, IFNULL(gm.guildid, 0) FROM characters AS c LEFT JOIN guild_member AS gm ON c.guid = gm.guid WHERE c.guid = ? AND c.deleteInfos_Name IS NULL"
	case StmtSelectDisplayCharacterByAccount:
		return "SELECT c.guid, c.name, account, race, class, gender, level, zone, map, position_x, position_y, position_z, IFNULL(gm.guildid, 0) FROM characters AS c LEFT JOIN guild_member AS gm ON c.guid = gm.guid WHERE c.account = ? AND c.deleteInfos_Name IS NULL ORDER BY c.guid LIMIT 1"
	case StmtUpdateCharacterPosition:
		return "UPDATE characters SET map = ?, position_x = ?, position_y = ?, position_z = ?, orientation = ? WHERE guid = ?"
	case StmtUpsertLfgDungeonRoute:
		return `INSERT INTO tc9_lfg_dungeon_routes
			(realmId, playerGuid, dungeonEntry, mapId, difficulty, ownerRealmId, isCrossRealm, requiresBoundInstance, instanceId, createdAt, updatedAt)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, UNIX_TIMESTAMP(), UNIX_TIMESTAMP())
			ON DUPLICATE KEY UPDATE
				dungeonEntry = VALUES(dungeonEntry),
				ownerRealmId = VALUES(ownerRealmId),
				isCrossRealm = VALUES(isCrossRealm),
				requiresBoundInstance = VALUES(requiresBoundInstance),
				instanceId = VALUES(instanceId),
				updatedAt = UNIX_TIMESTAMP()`
	case StmtConfirmLfgDungeonRouteEntered:
		return `UPDATE tc9_lfg_dungeon_routes AS route
				LEFT JOIN (
					SELECT ci.guid, MAX(ci.instance) AS instance, i.map, i.difficulty
					FROM character_instance AS ci
					JOIN instance AS i ON i.id = ci.instance
					WHERE ci.guid = ? AND i.map = ?
					GROUP BY ci.guid, i.map, i.difficulty
				) AS bind ON bind.guid = route.playerGuid
					AND bind.map = route.mapId
					AND bind.difficulty = route.difficulty
				SET route.requiresBoundInstance = 1,
					route.instanceId = IFNULL(bind.instance, route.instanceId),
					route.updatedAt = UNIX_TIMESTAMP()
				WHERE route.realmId = ? AND route.playerGuid = ? AND route.mapId = ? AND route.difficulty = ?`
	case StmtClearUnboundLfgDungeonRoute:
		return `DELETE FROM tc9_lfg_dungeon_routes
			WHERE realmId = ? AND playerGuid = ? AND (? = 0 OR mapId = ?) AND requiresBoundInstance = 0 AND instanceId = 0`
	case StmtSelectLfgDungeonRoute:
		return `SELECT route.realmId, route.playerGuid, route.dungeonEntry, route.mapId, route.difficulty,
				route.ownerRealmId, route.isCrossRealm, route.requiresBoundInstance, route.instanceId,
				IFNULL(bind.instance, 0) AS boundInstanceId
				FROM tc9_lfg_dungeon_routes AS route
				LEFT JOIN (
					SELECT ci.guid, MAX(ci.instance) AS instance, i.map, i.difficulty
					FROM character_instance AS ci
					JOIN instance AS i ON i.id = ci.instance
					WHERE ci.guid = ?
					GROUP BY ci.guid, i.map, i.difficulty
				) AS bind ON bind.guid = route.playerGuid
					AND bind.map = route.mapId
					AND bind.difficulty = route.difficulty
				WHERE route.realmId = ? AND route.playerGuid = ? AND (? = 0 OR route.mapId = ?)
				ORDER BY route.updatedAt DESC`
	case StmtSelectLfgDungeonBoundInstance:
		return `SELECT IFNULL(MAX(ci.instance), 0)
				FROM character_instance AS ci
				JOIN instance AS i ON i.id = ci.instance
				WHERE ci.guid = ? AND i.map = ? AND i.difficulty = ?`
	case StmtUpdateLfgDungeonRouteInstance:
		return `UPDATE tc9_lfg_dungeon_routes
				SET requiresBoundInstance = 1, instanceId = ?, updatedAt = UNIX_TIMESTAMP()
				WHERE realmId = ? AND playerGuid = ? AND mapId = ? AND difficulty = ?`
	case StmtSelectLfgDungeonRouteByInstance:
		return `SELECT route.realmId, route.playerGuid, route.dungeonEntry, route.mapId, route.difficulty,
				route.ownerRealmId, route.isCrossRealm, route.requiresBoundInstance, route.instanceId,
				route.instanceId AS boundInstanceId
				FROM tc9_lfg_dungeon_routes AS route
				WHERE route.mapId = ? AND route.difficulty = ? AND route.instanceId = ? AND route.requiresBoundInstance = 1 AND route.dungeonEntry <> 0
				ORDER BY route.updatedAt DESC
				LIMIT 1`
	case StmtGetFriendsForPlayer:
		return "SELECT friend, flags, note FROM character_social WHERE guid = ?"
	case StmtAddFriend:
		return "INSERT INTO character_social (guid, friend, flags, note) VALUES (?, ?, ?, ?)"
	case StmtRemoveFriend:
		return "DELETE FROM character_social WHERE guid = ? AND friend = ? AND flags = ?"
	case StmtUpdateFriendNote:
		return "UPDATE character_social SET note = ? WHERE guid = ? AND friend = ? AND flags = ?"
	case StmtAddIgnore:
		return "INSERT INTO character_social (guid, friend, flags, note) VALUES (?, ?, ?, '')"
	case StmtRemoveIgnore:
		return "DELETE FROM character_social WHERE guid = ? AND friend = ? AND flags = ?"
	case StmtGetPlayersWhoHaveAsFriend:
		return "SELECT guid FROM character_social WHERE friend = ? AND flags = ?"
	}

	panic(fmt.Errorf("unk stmt %d", s))
}

func (s CharactersPreparedStatements) ID() uint32 {
	return uint32(s)
}

const (
	StmtListCharactersToLogin CharactersPreparedStatements = iota
	StmtCharacterToLogin
	StmtSelectAccountData
	StmtReplaceAccountData
	StmtSelectCharacterWithGUID
	StmtSelectCharacterWithName
	StmtSelectDisplayCharacterByAccount
	StmtUpdateCharacterPosition
	StmtUpsertLfgDungeonRoute
	StmtConfirmLfgDungeonRouteEntered
	StmtClearUnboundLfgDungeonRoute
	StmtSelectLfgDungeonRoute
	StmtSelectLfgDungeonBoundInstance
	StmtUpdateLfgDungeonRouteInstance
	StmtSelectLfgDungeonRouteByInstance
	StmtGetFriendsForPlayer
	StmtAddFriend
	StmtRemoveFriend
	StmtUpdateFriendNote
	StmtAddIgnore
	StmtRemoveIgnore
	StmtGetPlayersWhoHaveAsFriend
)

type CharactersMYSQL struct {
	db shrepo.CharactersDB
}

func NewCharactersMYSQL(db shrepo.CharactersDB) Characters {
	db.SetPreparedStatement(StmtListCharactersToLogin)
	db.SetPreparedStatement(StmtSelectAccountData)
	db.SetPreparedStatement(StmtReplaceAccountData)
	db.SetPreparedStatement(StmtCharacterToLogin)
	db.SetPreparedStatement(StmtSelectCharacterWithGUID)
	db.SetPreparedStatement(StmtSelectCharacterWithName)
	db.SetPreparedStatement(StmtSelectDisplayCharacterByAccount)
	db.SetPreparedStatement(StmtUpdateCharacterPosition)
	db.SetPreparedStatement(StmtUpsertLfgDungeonRoute)
	db.SetPreparedStatement(StmtConfirmLfgDungeonRouteEntered)
	db.SetPreparedStatement(StmtClearUnboundLfgDungeonRoute)
	db.SetPreparedStatement(StmtSelectLfgDungeonRoute)
	db.SetPreparedStatement(StmtSelectLfgDungeonBoundInstance)
	db.SetPreparedStatement(StmtUpdateLfgDungeonRouteInstance)
	db.SetPreparedStatement(StmtSelectLfgDungeonRouteByInstance)
	db.SetPreparedStatement(StmtGetFriendsForPlayer)
	db.SetPreparedStatement(StmtAddFriend)
	db.SetPreparedStatement(StmtRemoveFriend)
	db.SetPreparedStatement(StmtUpdateFriendNote)
	db.SetPreparedStatement(StmtAddIgnore)
	db.SetPreparedStatement(StmtRemoveIgnore)
	db.SetPreparedStatement(StmtGetPlayersWhoHaveAsFriend)

	return &CharactersMYSQL{
		db: db,
	}
}

func (c CharactersMYSQL) ListCharactersToLogIn(ctx context.Context, realmID, accountID uint32) ([]LogInCharacter, error) {
	const currentPetSlot = 0
	rows, err := c.db.PreparedStatement(realmID, StmtListCharactersToLogin).QueryContext(ctx, currentPetSlot, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []LogInCharacter{}
	for rows.Next() {
		var equipmentCache string
		var bannedGuid uint32

		item := LogInCharacter{}
		err = rows.Scan(
			&item.GUID, &item.AccountID, &item.Name, &item.Race, &item.Class, &item.Gender, &item.Skin, &item.Face, &item.HairStyle,
			&item.HairColor, &item.FacialStyle, &item.Level, &item.Zone, &item.Map, &item.PositionX, &item.PositionY, &item.PositionZ,
			&item.GuildID, &item.PlayerFlags, &item.ExtraFlags, &item.AtLoginFlags, &item.PetEntry, &item.PetModelID, &item.PetLevel,
			&equipmentCache, &bannedGuid,
		)
		if err != nil {
			return nil, err
		}

		if bannedGuid > 0 {
			item.Banned = true
		}

		strs := strings.Split(equipmentCache, " ")
		item.Equipments = make([]uint32, 23)
		item.Enchants = make([]uint32, 23)

		for i, j := 0, 0; j < 23; i, j = i+2, j+1 {
			equip, _ := strconv.Atoi(strs[i])
			item.Equipments[j] = uint32(equip)

			if i+1 < len(strs) {
				enchant, _ := strconv.Atoi(strs[i+1])
				item.Enchants[j] = uint32(enchant)
			}
		}

		result = append(result, item)
	}

	return result, nil
}

func (c CharactersMYSQL) CharacterToLogInByGUID(ctx context.Context, realmID, accountID uint32, charGUID uint64) (*LogInCharacter, error) {
	const currentPetSlot = 0
	rows, err := c.db.PreparedStatement(realmID, StmtCharacterToLogin).QueryContext(ctx, currentPetSlot, charGUID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []LogInCharacter{}
	for rows.Next() {
		var equipmentCache string
		var bannedGuid uint32

		item := LogInCharacter{}
		err = rows.Scan(
			&item.GUID, &item.AccountID, &item.Name, &item.Race, &item.Class, &item.Gender, &item.Skin, &item.Face, &item.HairStyle,
			&item.HairColor, &item.FacialStyle, &item.Level, &item.Zone, &item.Map, &item.PositionX, &item.PositionY, &item.PositionZ,
			&item.GuildID, &item.PlayerFlags, &item.ExtraFlags, &item.AtLoginFlags, &item.PetEntry, &item.PetModelID, &item.PetLevel,
			&equipmentCache, &bannedGuid,
		)
		if err != nil {
			return nil, err
		}

		if bannedGuid > 0 {
			item.Banned = true
		}

		strs := strings.Split(equipmentCache, " ")
		item.Equipments = make([]uint32, 23)
		item.Enchants = make([]uint32, 23)

		for i, j := 0, 0; j < 23; i, j = i+2, j+1 {
			equip, _ := strconv.Atoi(strs[i])
			item.Equipments[j] = uint32(equip)

			if i+1 < len(strs) {
				enchant, _ := strconv.Atoi(strs[i+1])
				item.Enchants[j] = uint32(enchant)
			}
		}

		result = append(result, item)
	}

	if len(result) == 0 {
		return nil, nil
	}

	return &result[0], nil
}

func (c CharactersMYSQL) CharacterByName(ctx context.Context, realmID uint32, name string) (*Character, error) {
	nameRunes := []rune(strings.ToLower(name))
	if len(nameRunes) == 0 {
		return nil, nil
	}

	nameRunes[0] = unicode.ToUpper(nameRunes[0])

	rows, err := c.db.PreparedStatement(realmID, StmtSelectCharacterWithName).QueryContext(ctx, string(nameRunes))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Character{}
	for rows.Next() {
		item := Character{
			RealmID:  realmID,
			CharName: string(nameRunes),
		}
		if err = rows.Scan(
			&item.CharGUID, &item.AccountID, &item.CharRace, &item.CharClass, &item.CharGender, &item.CharLevel,
			&item.CharZone, &item.CharMap, &item.CharPosX, &item.CharPosY, &item.CharPosZ, &item.CharGuildID,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	if len(result) == 0 {
		return nil, nil
	}

	return &result[0], nil

}

func (c CharactersMYSQL) CharacterByGUID(ctx context.Context, realmID uint32, charGUID uint64) (*Character, error) {
	rows, err := c.db.PreparedStatement(realmID, StmtSelectCharacterWithGUID).QueryContext(ctx, charGUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Character{}
	for rows.Next() {
		item := Character{
			RealmID: realmID,
		}
		if err = rows.Scan(
			&item.CharGUID, &item.CharName, &item.AccountID, &item.CharRace, &item.CharClass, &item.CharGender, &item.CharLevel,
			&item.CharZone, &item.CharMap, &item.CharPosX, &item.CharPosY, &item.CharPosZ, &item.CharGuildID,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	if len(result) == 0 {
		return nil, nil
	}

	return &result[0], nil
}

func (c CharactersMYSQL) DisplayCharacterByAccount(ctx context.Context, accountID uint32) (*Character, error) {
	for _, realmID := range c.db.RealmIDs() {
		rows, err := c.db.PreparedStatement(realmID, StmtSelectDisplayCharacterByAccount).QueryContext(ctx, accountID)
		if err != nil {
			return nil, err
		}

		var result *Character
		if rows.Next() {
			item := Character{
				RealmID: realmID,
			}
			if err = rows.Scan(
				&item.CharGUID, &item.CharName, &item.AccountID, &item.CharRace, &item.CharClass, &item.CharGender, &item.CharLevel,
				&item.CharZone, &item.CharMap, &item.CharPosX, &item.CharPosY, &item.CharPosZ, &item.CharGuildID,
			); err != nil {
				_ = rows.Close()
				return nil, err
			}
			result = &item
		}

		if err = rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err = rows.Close(); err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	return nil, nil
}

func (c CharactersMYSQL) AccountDataForAccountID(ctx context.Context, realmID, accountID uint32) ([]AccountData, error) {
	rows, err := c.db.PreparedStatement(realmID, StmtSelectAccountData).QueryContext(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []AccountData{}
	for rows.Next() {
		item := AccountData{}
		if err = rows.Scan(&item.Type, &item.Time, &item.Data); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, nil
}

func (c CharactersMYSQL) UpdateAccountDataForAccountID(ctx context.Context, realmID, accountID uint32, data AccountData) error {
	_, err := c.db.PreparedStatement(realmID, StmtReplaceAccountData).ExecContext(ctx, accountID, data.Type, data.Time, data.Data)
	return err
}

func (c CharactersMYSQL) SaveCharacterPosition(ctx context.Context, realmID uint32, charGUID uint64, mapID uint32, x, y, z, o float32) error {
	_, err := c.db.PreparedStatement(realmID, StmtUpdateCharacterPosition).ExecContext(ctx, mapID, x, y, z, o, charGUID)
	if err != nil {
		return err
	}

	return nil
}

func (c CharactersMYSQL) RecordLfgDungeonRoute(ctx context.Context, route LfgDungeonRoute) error {
	if route.RealmID == 0 {
		route.RealmID = 1
	}
	if route.PlayerGUID == 0 || route.MapID == 0 || route.DungeonEntry == 0 {
		return nil
	}
	if !route.RequiresBoundInstance {
		existing, err := c.LfgDungeonRouteForPlayer(ctx, route.RealmID, route.PlayerGUID, route.MapID)
		if err != nil {
			return err
		}
		if existing != nil && existing.Difficulty == route.Difficulty && existing.RequiresBoundInstance && existing.BoundInstanceID != 0 {
			route.RequiresBoundInstance = true
			route.InstanceID = existing.InstanceID
			if route.InstanceID == 0 {
				route.InstanceID = existing.BoundInstanceID
			}
		}
	}

	_, err := c.db.PreparedStatement(route.RealmID, StmtUpsertLfgDungeonRoute).ExecContext(
		ctx,
		route.RealmID,
		route.PlayerGUID,
		route.DungeonEntry,
		route.MapID,
		route.Difficulty,
		route.OwnerRealmID,
		route.IsCrossRealm,
		route.RequiresBoundInstance,
		route.InstanceID,
	)
	return err
}

func (c CharactersMYSQL) ConfirmLfgDungeonRouteEntered(ctx context.Context, realmID uint32, playerGUID uint64, mapID uint32, difficulty uint8, instanceID uint32) (*LfgDungeonRoute, error) {
	if playerGUID == 0 || mapID == 0 {
		return nil, nil
	}
	var err error
	if instanceID != 0 {
		_, err = c.db.PreparedStatement(realmID, StmtUpdateLfgDungeonRouteInstance).ExecContext(ctx, instanceID, realmID, playerGUID, mapID, difficulty)
	} else {
		_, err = c.db.PreparedStatement(realmID, StmtConfirmLfgDungeonRouteEntered).ExecContext(ctx, playerGUID, mapID, realmID, playerGUID, mapID, difficulty)
	}
	if err != nil {
		return nil, err
	}
	route, err := c.LfgDungeonRouteForPlayer(ctx, realmID, playerGUID, mapID)
	if err != nil || route != nil || instanceID == 0 {
		return route, err
	}

	template, err := c.lfgDungeonRouteTemplateForInstance(ctx, realmID, mapID, difficulty, instanceID)
	if err != nil || template == nil {
		return nil, err
	}

	template.RealmID = realmID
	template.PlayerGUID = playerGUID
	template.RequiresBoundInstance = true
	template.InstanceID = instanceID
	template.BoundInstanceID = instanceID
	if err = c.RecordLfgDungeonRoute(ctx, *template); err != nil {
		return nil, err
	}
	return c.LfgDungeonRouteForPlayer(ctx, realmID, playerGUID, mapID)
}

func (c CharactersMYSQL) ClearUnboundLfgDungeonRoute(ctx context.Context, realmID uint32, playerGUID uint64, mapID uint32) error {
	if playerGUID == 0 {
		return nil
	}
	_, err := c.db.PreparedStatement(realmID, StmtClearUnboundLfgDungeonRoute).ExecContext(ctx, realmID, playerGUID, mapID, mapID)
	return err
}

func (c CharactersMYSQL) LfgDungeonRouteForPlayer(ctx context.Context, realmID uint32, playerGUID uint64, mapID uint32) (*LfgDungeonRoute, error) {
	if playerGUID == 0 {
		return nil, nil
	}

	rows, err := c.db.PreparedStatement(realmID, StmtSelectLfgDungeonRoute).QueryContext(ctx, playerGUID, realmID, playerGUID, mapID, mapID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []LfgDungeonRoute
	for rows.Next() {
		route := LfgDungeonRoute{}
		if err = rows.Scan(
			&route.RealmID,
			&route.PlayerGUID,
			&route.DungeonEntry,
			&route.MapID,
			&route.Difficulty,
			&route.OwnerRealmID,
			&route.IsCrossRealm,
			&route.RequiresBoundInstance,
			&route.InstanceID,
			&route.BoundInstanceID,
		); err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, nil
	}

	for _, route := range routes {
		if route.RequiresBoundInstance && route.BoundInstanceID == 0 {
			route.BoundInstanceID, err = c.findLfgDungeonRouteBoundInstance(ctx, realmID, route)
			if err != nil {
				return nil, err
			}
		}
		if route.RequiresBoundInstance && route.BoundInstanceID != 0 && route.InstanceID != route.BoundInstanceID {
			_, err = c.db.PreparedStatement(realmID, StmtUpdateLfgDungeonRouteInstance).ExecContext(ctx, route.BoundInstanceID, route.RealmID, route.PlayerGUID, route.MapID, route.Difficulty)
			if err != nil {
				return nil, err
			}
			route.InstanceID = route.BoundInstanceID
		}
		return &route, nil
	}

	return nil, nil
}

func (c CharactersMYSQL) findLfgDungeonRouteBoundInstance(ctx context.Context, requestRealmID uint32, route LfgDungeonRoute) (uint32, error) {
	for _, realmID := range c.lfgDungeonRouteBoundInstanceRealmIDs(requestRealmID, route) {
		instanceID, err := c.lfgDungeonBoundInstanceInRealm(ctx, realmID, route.PlayerGUID, route.MapID, route.Difficulty)
		if err != nil {
			return 0, err
		}
		if instanceID != 0 {
			return instanceID, nil
		}
	}

	return 0, nil
}

func (c CharactersMYSQL) lfgDungeonRouteBoundInstanceRealmIDs(requestRealmID uint32, route LfgDungeonRoute) []uint32 {
	configuredRealms := c.db.RealmIDs()
	configuredRealmSet := make(map[uint32]struct{}, len(configuredRealms))
	for _, realmID := range configuredRealms {
		configuredRealmSet[realmID] = struct{}{}
	}

	realmIDSet := make(map[uint32]struct{})
	realmIDs := make([]uint32, 0, 4)
	addRealm := func(realmID uint32) {
		if realmID == 0 {
			return
		}
		if _, ok := realmIDSet[realmID]; ok {
			return
		}
		realmIDSet[realmID] = struct{}{}
		realmIDs = append(realmIDs, realmID)
	}

	addRealm(route.RealmID)
	addRealm(requestRealmID)
	if _, ok := configuredRealmSet[route.OwnerRealmID]; ok {
		addRealm(route.OwnerRealmID)
	}

	if route.IsCrossRealm && route.OwnerRealmID == 0 {
		for _, realmID := range configuredRealms {
			addRealm(realmID)
		}
	}

	return realmIDs
}

func (c CharactersMYSQL) lfgDungeonBoundInstanceInRealm(ctx context.Context, realmID uint32, playerGUID uint64, mapID uint32, difficulty uint8) (uint32, error) {
	var instanceID uint32
	err := c.db.PreparedStatement(realmID, StmtSelectLfgDungeonBoundInstance).QueryRowContext(ctx, playerGUID, mapID, difficulty).Scan(&instanceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return instanceID, nil
}

func (c CharactersMYSQL) lfgDungeonRouteTemplateForInstance(ctx context.Context, requestRealmID uint32, mapID uint32, difficulty uint8, instanceID uint32) (*LfgDungeonRoute, error) {
	route, err := c.lfgDungeonRouteTemplateForInstanceID(ctx, requestRealmID, mapID, difficulty, instanceID)
	if err != nil || route != nil || instanceID == 0 {
		return route, err
	}
	return c.lfgDungeonRouteTemplateForInstanceID(ctx, requestRealmID, mapID, difficulty, 0)
}

func (c CharactersMYSQL) lfgDungeonRouteTemplateForInstanceID(ctx context.Context, requestRealmID uint32, mapID uint32, difficulty uint8, instanceID uint32) (*LfgDungeonRoute, error) {
	seen := make(map[uint32]struct{})
	realmIDs := append([]uint32{requestRealmID}, c.db.RealmIDs()...)
	for _, realmID := range realmIDs {
		if realmID == 0 {
			continue
		}
		if _, ok := seen[realmID]; ok {
			continue
		}
		seen[realmID] = struct{}{}

		route := LfgDungeonRoute{}
		err := c.db.PreparedStatement(realmID, StmtSelectLfgDungeonRouteByInstance).QueryRowContext(ctx, mapID, difficulty, instanceID).Scan(
			&route.RealmID,
			&route.PlayerGUID,
			&route.DungeonEntry,
			&route.MapID,
			&route.Difficulty,
			&route.OwnerRealmID,
			&route.IsCrossRealm,
			&route.RequiresBoundInstance,
			&route.InstanceID,
			&route.BoundInstanceID,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return nil, err
		}
		return &route, nil
	}

	return nil, nil
}

func (c CharactersMYSQL) GetFriendsForPlayer(ctx context.Context, realmID uint32, playerGUID uint64) ([]*FriendEntry, error) {
	rows, err := c.db.PreparedStatement(realmID, StmtGetFriendsForPlayer).QueryContext(ctx, playerGUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*FriendEntry
	for rows.Next() {
		entry := &FriendEntry{PlayerRealmID: realmID, PlayerGUID: playerGUID}
		err = rows.Scan(&entry.FriendGUID, &entry.Flags, &entry.Note)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}

	return result, rows.Err()
}

func (c CharactersMYSQL) AddFriend(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64, note string) error {
	_, err := c.db.PreparedStatement(realmID, StmtAddFriend).ExecContext(ctx, playerGUID, friendGUID, SocialFlagFriend, note)
	return err
}

func (c CharactersMYSQL) RemoveFriend(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64) error {
	_, err := c.db.PreparedStatement(realmID, StmtRemoveFriend).ExecContext(ctx, playerGUID, friendGUID, SocialFlagFriend)
	return err
}

func (c CharactersMYSQL) UpdateFriendNote(ctx context.Context, realmID uint32, playerGUID, friendGUID uint64, note string) error {
	_, err := c.db.PreparedStatement(realmID, StmtUpdateFriendNote).ExecContext(ctx, note, playerGUID, friendGUID, SocialFlagFriend)
	return err
}

func (c CharactersMYSQL) AddIgnore(ctx context.Context, realmID uint32, playerGUID, ignoredGUID uint64) error {
	_, err := c.db.PreparedStatement(realmID, StmtAddIgnore).ExecContext(ctx, playerGUID, ignoredGUID, SocialFlagIgnore)
	return err
}

func (c CharactersMYSQL) RemoveIgnore(ctx context.Context, realmID uint32, playerGUID, ignoredGUID uint64) error {
	_, err := c.db.PreparedStatement(realmID, StmtRemoveIgnore).ExecContext(ctx, playerGUID, ignoredGUID, SocialFlagIgnore)
	return err
}

func (c CharactersMYSQL) GetPlayersWhoHaveAsFriend(ctx context.Context, realmID uint32, playerGUID uint64) ([]uint64, error) {
	rows, err := c.db.PreparedStatement(realmID, StmtGetPlayersWhoHaveAsFriend).QueryContext(ctx, playerGUID, SocialFlagFriend)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []uint64
	for rows.Next() {
		var guid uint64
		err = rows.Scan(&guid)
		if err != nil {
			return nil, err
		}
		result = append(result, guid)
	}

	return result, rows.Err()
}
