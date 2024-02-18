package repo

import (
	"context"
	"strconv"
	"strings"
	"unicode"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

type CharactersMYSQL struct {
	db shrepo.CharactersDB
}

func NewCharactersMYSQL(db shrepo.CharactersDB, schemaType shrepo.SupportedSchemaType) Characters {
	db.SetPreparedStatement(StmtListCharactersToLogin.SchemeStatement(schemaType))
	db.SetPreparedStatement(StmtSelectAccountData.SchemeStatement(schemaType))
	db.SetPreparedStatement(StmtCharacterToLogin.SchemeStatement(schemaType))
	db.SetPreparedStatement(StmtSelectCharacterWithName.SchemeStatement(schemaType))

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
			&item.GuildID, &item.PlayerFlags, &item.AtLoginFlags, &item.PetEntry, &item.PetModelID, &item.PetLevel,
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

		for i, j := 0, 0; j < 23 && i < len(strs); i, j = i+2, j+1 {
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

func (c CharactersMYSQL) CharacterToLogInByGUID(ctx context.Context, realmID uint32, charGUID uint64) (*LogInCharacter, error) {
	const currentPetSlot = 0
	rows, err := c.db.PreparedStatement(realmID, StmtCharacterToLogin).QueryContext(ctx, currentPetSlot, charGUID)
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
			&item.GuildID, &item.PlayerFlags, &item.AtLoginFlags, &item.PetEntry, &item.PetModelID, &item.PetLevel,
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

		for i, j := 0, 0; j < 23 && i < len(strs); i, j = i+2, j+1 {
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
