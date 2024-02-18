package repo

import (
	"fmt"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

type CharactersPreparedStatements uint32

const (
	StmtListCharactersToLogin CharactersPreparedStatements = iota
	StmtCharacterToLogin
	StmtSelectAccountData
	StmtSelectCharacterWithName
)

func (s CharactersPreparedStatements) ID() uint32 {
	return uint32(s)
}

func (s CharactersPreparedStatements) SchemeStatement(schemaType shrepo.SupportedSchemaType) shrepo.PreparedStatement {
	switch schemaType {
	case shrepo.SupportedSchemaTypeTrinityCore, shrepo.SupportedSchemaTypeAzerothCore:
		return shrepo.NewGenericPreparedStatement(s.ID(), s.tcAndAcScheme())
	case shrepo.SupportedSchemaTypeCMaNGOS:
		return shrepo.NewGenericPreparedStatement(s.ID(), s.cmangosScheme())
	}

	panic(fmt.Errorf("unk scheme %s", schemaType))
}

func (s CharactersPreparedStatements) tcAndAcScheme() string {
	switch s {
	case StmtListCharactersToLogin:
		return `SELECT c.guid, c.account, c.name, c.race, c.class, c.gender, c.skin, c.face, c.hairStyle, c.hairColor, c.facialStyle, c.level, c.zone, c.map, c.position_x, c.position_y, c.position_z, 
		IFNULL(gm.guildid, 0), c.playerFlags, c.at_login, IFNULL(cp.entry, 0), IFNULL(cp.modelid, 0), IFNULL(cp.level, 0), c.equipmentCache, IFNULL(cb.guid, 0) 
		FROM characters AS c LEFT JOIN character_pet AS cp ON c.guid = cp.owner AND cp.slot = ? LEFT JOIN guild_member AS gm ON c.guid = gm.guid 
		LEFT JOIN character_banned AS cb ON c.guid = cb.guid AND cb.active = 1 WHERE c.account = ? AND c.deleteInfos_Name IS NULL ORDER BY c.guid`
	case StmtCharacterToLogin:
		return `SELECT c.guid, c.account, c.name, c.race, c.class, c.gender, c.skin, c.face, c.hairStyle, c.hairColor, c.facialStyle, c.level, c.zone, c.map, c.position_x, c.position_y, c.position_z, 
		IFNULL(gm.guildid, 0), c.playerFlags, c.at_login, IFNULL(cp.entry, 0), IFNULL(cp.modelid, 0), IFNULL(cp.level, 0), c.equipmentCache, IFNULL(cb.guid, 0) 
		FROM characters AS c LEFT JOIN character_pet AS cp ON c.guid = cp.owner AND cp.slot = ? LEFT JOIN guild_member AS gm ON c.guid = gm.guid 
		LEFT JOIN character_banned AS cb ON c.guid = cb.guid AND cb.active = 1 WHERE c.guid = ? AND c.deleteInfos_Name IS NULL`
	case StmtSelectAccountData:
		return "SELECT type, time, data FROM account_data WHERE accountId = ?"
	case StmtSelectCharacterWithName:
		return "SELECT c.guid, account, race, class, gender, level, zone, map, position_x, position_y, position_z, IFNULL(gm.guildid, 0) FROM characters AS c LEFT JOIN guild_member AS gm ON c.guid = gm.guid WHERE name = ?"
	}

	panic(fmt.Errorf("unk stmt %d", s))
}

func (s CharactersPreparedStatements) cmangosScheme() string {
	switch s {
	case StmtListCharactersToLogin:
		return `SELECT c.guid, c.account, c.name, c.race, c.class, c.gender, 
        (c.playerBytes & 255) AS skin, ((c.playerBytes >> 8) & 255) AS face, ((c.playerBytes >> 16) & 255) AS hairStyle, 
        ((c.playerBytes >> 24) & 255) AS hairColor, (c.playerBytes2 & 255) AS facialStyle, 
        c.level, c.zone, c.map, c.position_x, c.position_y, c.position_z, IFNULL(gm.guildid, 0), c.playerFlags, c.at_login, IFNULL(cp.entry, 0), IFNULL(cp.modelid, 0), IFNULL(cp.level, 0), c.equipmentCache, 0
		FROM characters AS c LEFT JOIN character_pet AS cp ON c.guid = cp.owner AND cp.slot = ? LEFT JOIN guild_member AS gm ON c.guid = gm.guid 
		WHERE c.account = ? AND c.deleteInfos_Name IS NULL ORDER BY c.guid`
	case StmtCharacterToLogin:
		return `SELECT c.guid, c.account, c.name, c.race, c.class, c.gender, 
        (c.playerBytes & 255) AS skin, ((c.playerBytes >> 8) & 255) AS face, ((c.playerBytes >> 16) & 255) AS hairStyle, 
        ((c.playerBytes >> 24) & 255) AS hairColor, (c.playerBytes2 & 255) AS facialStyle, 
        c.level, c.zone, c.map, c.position_x, c.position_y, c.position_z, IFNULL(gm.guildid, 0), c.playerFlags, c.at_login, IFNULL(cp.entry, 0), IFNULL(cp.modelid, 0), IFNULL(cp.level, 0), c.equipmentCache, 0
		FROM characters AS c LEFT JOIN character_pet AS cp ON c.guid = cp.owner AND cp.slot = ? LEFT JOIN guild_member AS gm ON c.guid = gm.guid 
		WHERE c.guid = ? AND c.deleteInfos_Name IS NULL`
	case StmtSelectAccountData:
		return "SELECT type, time, data FROM account_data WHERE account = ?"
	case StmtSelectCharacterWithName:
		return "SELECT c.guid, account, race, class, gender, level, zone, map, position_x, position_y, position_z, IFNULL(gm.guildid, 0) FROM characters AS c LEFT JOIN guild_member AS gm ON c.guid = gm.guid WHERE name = ?"
	}

	panic(fmt.Errorf("unk stmt %d", s))
}
