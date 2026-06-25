package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

var (
	ErrArenaTeamNotFound       = errors.New("arena team not found")
	ErrArenaTeamMemberMismatch = errors.New("arena team member mismatch")
	ErrArenaTeamPartySize      = errors.New("invalid arena team party size")
	ErrArenaTeamNotOwner       = errors.New("arena petition is not owned by captain")
	ErrArenaTeamInvalidType    = errors.New("invalid arena team type")
	ErrArenaTeamNameExists     = errors.New("arena team name exists")
	ErrArenaTeamAlreadyInTeam  = errors.New("arena team member already belongs to bracket")
	ErrArenaTeamNotEnoughSigns = errors.New("arena petition has insufficient signatures")
	ErrArenaTeamRosterFull     = errors.New("arena team roster is full")
	ErrArenaTeamInvalidName    = errors.New("invalid arena team name")
	ErrArenaTeamPermission     = errors.New("arena team permission denied")
	ErrArenaTeamLeaderLeave    = errors.New("arena team leader cannot leave")
)

const (
	minArenaTeamNameRunes = 2
	maxArenaTeamNameRunes = 24
)

type ArenaTeamQueueData struct {
	ArenaTeamID             uint32
	TeamRating              uint32
	MatchmakerRating        uint32
	PreviousOpponentsTeamID uint32
}

type ArenaTeamCreateFromPetitionRequest struct {
	RealmID               uint32
	CaptainGUID           uint64
	PetitionGUID          uint64
	ArenaType             uint8
	BackgroundColor       uint32
	EmblemStyle           uint8
	EmblemColor           uint32
	BorderStyle           uint8
	BorderColor           uint32
	StartRating           uint32
	StartPersonalRating   uint32
	StartMatchmakerRating uint32
}

type ArenaTeamCreateFromPetitionResult struct {
	ArenaTeamID uint32
}

type ArenaTeamPetitionDetails struct {
	PetitionGUID uint64
	PetitionID   uint32
	OwnerGUID    uint64
	Name         string
	ArenaType    uint8
	Signatures   uint32
}

type ArenaTeamDetails struct {
	ArenaTeamID     uint32
	Name            string
	CaptainGUID     uint64
	Type            uint8
	Rating          uint32
	WeekGames       uint32
	WeekWins        uint32
	SeasonGames     uint32
	SeasonWins      uint32
	Rank            uint32
	BackgroundColor uint32
	EmblemStyle     uint8
	EmblemColor     uint32
	BorderStyle     uint8
	BorderColor     uint32
	Members         []ArenaTeamDetailsMember
}

type ArenaTeamDetailsMember struct {
	PlayerGUID       uint64
	Name             string
	Level            uint8
	Class            uint8
	WeekGames        uint32
	WeekWins         uint32
	SeasonGames      uint32
	SeasonWins       uint32
	PersonalRating   uint32
	MatchmakerRating uint32
	MaxMMR           uint32
}

type ArenaTeamSaveStatsRequest struct {
	RealmID     uint32
	ArenaTeamID uint32
	Rating      uint32
	WeekGames   uint32
	WeekWins    uint32
	SeasonGames uint32
	SeasonWins  uint32
	Rank        uint32
	Slot        uint32
	Members     []ArenaTeamSaveStatsMember
}

type ArenaTeamSaveStatsMember struct {
	PlayerGUID       uint64
	PersonalRating   uint32
	WeekGames        uint32
	WeekWins         uint32
	SeasonGames      uint32
	SeasonWins       uint32
	MatchmakerRating uint32
	MaxMMR           uint32
	SaveArenaStats   bool
}

type ArenaTeams interface {
	QueueDataForRatedArena(ctx context.Context, realmID uint32, leaderGUID uint64, playerGUIDs []uint64, arenaType uint8, startMatchmakerRating uint32) (*ArenaTeamQueueData, error)
	GetTeam(ctx context.Context, realmID uint32, arenaTeamID uint32) (*ArenaTeamDetails, error)
	GetPetition(ctx context.Context, realmID uint32, petitionGUID uint64) (*ArenaTeamPetitionDetails, error)
	MemberTeamForType(ctx context.Context, realmID uint32, playerGUID uint64, arenaType uint8) (uint32, bool, error)
	CreateFromPetition(ctx context.Context, request ArenaTeamCreateFromPetitionRequest) (*ArenaTeamCreateFromPetitionResult, error)
	AddMember(ctx context.Context, realmID uint32, arenaTeamID uint32, playerGUID uint64, personalRating uint32) error
	RemoveMember(ctx context.Context, realmID uint32, arenaTeamID uint32, playerGUID uint64, actorGUID uint64) error
	Disband(ctx context.Context, realmID uint32, arenaTeamID uint32, actorGUID uint64) error
	SetCaptain(ctx context.Context, realmID uint32, arenaTeamID uint32, captainGUID uint64, actorGUID uint64) error
	SetName(ctx context.Context, realmID uint32, arenaTeamID uint32, name string, actorGUID uint64) error
	SaveStats(ctx context.Context, request ArenaTeamSaveStatsRequest) error
	DeleteAll(ctx context.Context, realmID uint32) error
	ValidateCharacterDelete(ctx context.Context, realmID uint32, playerGUID uint64) error
	RemovePlayerFromTeams(ctx context.Context, realmID uint32, playerGUID uint64) error
}

type ArenaTeamsMYSQL struct {
	db shrepo.CharactersDB
}

func NewArenaTeamsMYSQL(db shrepo.CharactersDB) ArenaTeams {
	return &ArenaTeamsMYSQL{db: db}
}

func (r *ArenaTeamsMYSQL) dbForRealm(realmID uint32) (*sql.DB, error) {
	db := r.db.DBByRealm(realmID)
	if db == nil {
		return nil, fmt.Errorf("characters db not configured for realm %d", realmID)
	}
	return db, nil
}

func (r *ArenaTeamsMYSQL) QueueDataForRatedArena(ctx context.Context, realmID uint32, leaderGUID uint64, playerGUIDs []uint64, arenaType uint8, startMatchmakerRating uint32) (*ArenaTeamQueueData, error) {
	slot, ok := arenaSlotByType(arenaType)
	if !ok {
		return nil, ErrArenaTeamInvalidType
	}

	if len(playerGUIDs) != int(arenaType) {
		return nil, ErrArenaTeamPartySize
	}

	db, err := r.dbForRealm(realmID)
	if err != nil {
		return nil, err
	}

	teamID, teamRating, err := r.teamForLeader(ctx, db, leaderGUID, arenaType)
	if err != nil {
		return nil, err
	}

	mmr, err := r.averageSelectedMembersMMR(ctx, db, teamID, playerGUIDs, slot, startMatchmakerRating)
	if err != nil {
		return nil, err
	}

	return &ArenaTeamQueueData{
		ArenaTeamID:             teamID,
		TeamRating:              teamRating,
		MatchmakerRating:        mmr,
		PreviousOpponentsTeamID: 0,
	}, nil
}

func (r *ArenaTeamsMYSQL) GetTeam(ctx context.Context, realmID uint32, arenaTeamID uint32) (*ArenaTeamDetails, error) {
	db, err := r.dbForRealm(realmID)
	if err != nil {
		return nil, err
	}

	team := ArenaTeamDetails{}
	err = db.QueryRowContext(ctx, `
SELECT arenaTeamId, name, captainGuid, type, rating, weekGames, weekWins, seasonGames, seasonWins, `+"`rank`"+`, backgroundColor, emblemStyle, emblemColor, borderStyle, borderColor
FROM arena_team
WHERE arenaTeamId = ?
LIMIT 1`, arenaTeamID).Scan(
		&team.ArenaTeamID,
		&team.Name,
		&team.CaptainGUID,
		&team.Type,
		&team.Rating,
		&team.WeekGames,
		&team.WeekWins,
		&team.SeasonGames,
		&team.SeasonWins,
		&team.Rank,
		&team.BackgroundColor,
		&team.EmblemStyle,
		&team.EmblemColor,
		&team.BorderStyle,
		&team.BorderColor,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrArenaTeamNotFound
		}
		return nil, err
	}
	slot, ok := arenaSlotByType(team.Type)
	if !ok {
		return nil, ErrArenaTeamInvalidType
	}

	rows, err := db.QueryContext(ctx, `
SELECT atm.guid, COALESCE(c.name, ''), COALESCE(c.level, 0), COALESCE(c.class, 0), atm.weekGames, atm.weekWins, atm.seasonGames, atm.seasonWins, atm.personalRating,
       COALESCE(NULLIF(cas.matchMakerRating, 0), 0), COALESCE(cas.maxMMR, 0)
FROM arena_team_member AS atm
LEFT JOIN characters AS c ON c.guid = atm.guid AND c.deleteInfos_Name IS NULL
LEFT JOIN character_arena_stats AS cas ON cas.guid = atm.guid AND cas.slot = ?
WHERE atm.arenaTeamId = ?
ORDER BY atm.guid ASC`, slot, arenaTeamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		member := ArenaTeamDetailsMember{}
		if err = rows.Scan(
			&member.PlayerGUID,
			&member.Name,
			&member.Level,
			&member.Class,
			&member.WeekGames,
			&member.WeekWins,
			&member.SeasonGames,
			&member.SeasonWins,
			&member.PersonalRating,
			&member.MatchmakerRating,
			&member.MaxMMR,
		); err != nil {
			return nil, err
		}
		team.Members = append(team.Members, member)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &team, nil
}

func (r *ArenaTeamsMYSQL) GetPetition(ctx context.Context, realmID uint32, petitionGUID uint64) (*ArenaTeamPetitionDetails, error) {
	db, err := r.dbForRealm(realmID)
	if err != nil {
		return nil, err
	}

	petitionGuidLow := petitionGUID
	if petitionGuidLow > 0xFFFFFFFF {
		petitionGuidLow &= 0xFFFFFFFF
	}

	petition, err := arenaPetitionByGUID(ctx, db, petitionGuidLow)
	if err != nil {
		return nil, err
	}
	if petition == nil {
		return nil, ErrArenaTeamNotFound
	}
	if _, ok := arenaSlotByType(petition.petitionType); !ok {
		return nil, ErrArenaTeamInvalidType
	}

	signatures, err := arenaPetitionSignatureCount(ctx, db, petition.petitionID)
	if err != nil {
		return nil, err
	}

	return &ArenaTeamPetitionDetails{
		PetitionGUID: petition.petitionGUID,
		PetitionID:   petition.petitionID,
		OwnerGUID:    petition.ownerGUID,
		Name:         petition.name,
		ArenaType:    petition.petitionType,
		Signatures:   signatures,
	}, nil
}

func (r *ArenaTeamsMYSQL) MemberTeamForType(ctx context.Context, realmID uint32, playerGUID uint64, arenaType uint8) (uint32, bool, error) {
	if _, ok := arenaSlotByType(arenaType); !ok {
		return 0, false, ErrArenaTeamInvalidType
	}

	db, err := r.dbForRealm(realmID)
	if err != nil {
		return 0, false, err
	}

	var arenaTeamID uint32
	err = db.QueryRowContext(ctx, `
SELECT atm.arenaTeamId
FROM arena_team_member AS atm
INNER JOIN arena_team AS at ON at.arenaTeamId = atm.arenaTeamId
WHERE atm.guid = ? AND at.type = ?
LIMIT 1`, playerGUID, arenaType).Scan(&arenaTeamID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return arenaTeamID, true, nil
}

func (r *ArenaTeamsMYSQL) teamForLeader(ctx context.Context, db *sql.DB, leaderGUID uint64, arenaType uint8) (uint32, uint32, error) {
	row := db.QueryRowContext(ctx, `
SELECT at.arenaTeamId, at.rating
FROM arena_team AS at
INNER JOIN arena_team_member AS atm ON atm.arenaTeamId = at.arenaTeamId
WHERE atm.guid = ? AND at.type = ?
LIMIT 1`, leaderGUID, arenaType)

	var teamID uint32
	var teamRating uint32
	if err := row.Scan(&teamID, &teamRating); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, ErrArenaTeamNotFound
		}
		return 0, 0, err
	}

	if teamRating == 0 {
		teamRating = 1
	}

	return teamID, teamRating, nil
}

func (r *ArenaTeamsMYSQL) averageSelectedMembersMMR(ctx context.Context, db *sql.DB, teamID uint32, playerGUIDs []uint64, slot uint8, startMatchmakerRating uint32) (uint32, error) {
	expected := make(map[uint64]struct{}, len(playerGUIDs))
	for _, playerGUID := range playerGUIDs {
		expected[playerGUID] = struct{}{}
	}

	rows, err := db.QueryContext(ctx, `
SELECT atm.guid, COALESCE(NULLIF(cas.matchMakerRating, 0), ?) AS matchMakerRating
FROM arena_team_member AS atm
LEFT JOIN character_arena_stats AS cas ON cas.guid = atm.guid AND cas.slot = ?
WHERE atm.arenaTeamId = ?`, startMatchmakerRating, slot, teamID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var total uint32
	var count uint32
	for rows.Next() {
		var memberGUID uint64
		var memberMMR uint32
		if err := rows.Scan(&memberGUID, &memberMMR); err != nil {
			return 0, err
		}

		if _, ok := expected[memberGUID]; !ok {
			continue
		}

		total += memberMMR
		count++
		delete(expected, memberGUID)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if len(expected) > 0 || count != uint32(len(playerGUIDs)) {
		return 0, ErrArenaTeamMemberMismatch
	}

	if count == 0 {
		return startMatchmakerRating, nil
	}

	return total / count, nil
}

func arenaSlotByType(arenaType uint8) (uint8, bool) {
	switch arenaType {
	case 2:
		return 0, true
	case 3:
		return 1, true
	case 5:
		return 2, true
	default:
		return 0, false
	}
}

func (r *ArenaTeamsMYSQL) CreateFromPetition(ctx context.Context, request ArenaTeamCreateFromPetitionRequest) (*ArenaTeamCreateFromPetitionResult, error) {
	_, ok := arenaSlotByType(request.ArenaType)
	if !ok {
		return nil, ErrArenaTeamInvalidType
	}

	db, err := r.dbForRealm(request.RealmID)
	if err != nil {
		return nil, err
	}

	petitionGuidLow := uint64(request.PetitionGUID)
	if petitionGuidLow > 0xFFFFFFFF {
		petitionGuidLow = petitionGuidLow & 0xFFFFFFFF
	}

	lockName := fmt.Sprintf("tc9_arena_team_create:%d", request.RealmID)
	var locked int
	if err := db.QueryRowContext(ctx, "SELECT GET_LOCK(?, 5)", lockName).Scan(&locked); err != nil {
		return nil, err
	}
	if locked != 1 {
		return nil, errors.New("cannot acquire arena team creation lock")
	}
	defer func() {
		_, _ = db.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", lockName)
	}()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	petition, err := arenaPetitionForUpdate(ctx, tx, petitionGuidLow)
	if err != nil {
		return nil, err
	}
	if petition == nil {
		return nil, ErrArenaTeamNotFound
	}
	if petition.ownerGUID != request.CaptainGUID {
		return nil, ErrArenaTeamNotOwner
	}
	if petition.petitionType != request.ArenaType {
		return nil, ErrArenaTeamInvalidType
	}
	if !isValidArenaTeamName(petition.name) {
		return nil, ErrArenaTeamInvalidName
	}

	exists, err := arenaTeamNameExists(ctx, tx, petition.name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrArenaTeamNameExists
	}

	signatures, err := arenaPetitionSignatures(ctx, tx, petition.petitionID)
	if err != nil {
		return nil, err
	}
	if len(signatures) < int(request.ArenaType)-1 {
		return nil, ErrArenaTeamNotEnoughSigns
	}

	memberGUIDs := arenaMemberGUIDsForCreate(request.CaptainGUID, signatures, request.ArenaType)

	if err = ensureArenaMembersAvailable(ctx, tx, memberGUIDs, request.ArenaType); err != nil {
		return nil, err
	}

	nextID, err := nextArenaTeamID(ctx, tx)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO arena_team (arenaTeamId, name, captainGuid, type, rating, backgroundColor, emblemStyle, emblemColor, borderStyle, borderColor)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nextID,
		petition.name,
		request.CaptainGUID,
		request.ArenaType,
		request.StartRating,
		request.BackgroundColor,
		request.EmblemStyle,
		request.EmblemColor,
		request.BorderStyle,
		request.BorderColor,
	)
	if err != nil {
		return nil, err
	}

	for _, memberGUID := range memberGUIDs {
		_, err = tx.ExecContext(ctx, `
INSERT INTO arena_team_member (arenaTeamId, guid, personalRating)
VALUES (?, ?, ?)`, nextID, memberGUID, request.StartPersonalRating)
		if err != nil {
			return nil, err
		}

		if err = removeArenaPetitionsAndSigns(ctx, tx, memberGUID, request.ArenaType); err != nil {
			return nil, err
		}
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM petition_sign WHERE petition_id = ?", petition.petitionID)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM petition WHERE petition_id = ?", petition.petitionID)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return &ArenaTeamCreateFromPetitionResult{ArenaTeamID: nextID}, nil
}

func (r *ArenaTeamsMYSQL) AddMember(ctx context.Context, realmID uint32, arenaTeamID uint32, playerGUID uint64, personalRating uint32) error {
	db, err := r.dbForRealm(realmID)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	arenaType, err := arenaTeamTypeForUpdate(ctx, tx, arenaTeamID)
	if err != nil {
		return err
	}

	existingTeamID, found, err := arenaMemberTeamForType(ctx, tx, playerGUID, arenaType)
	if err != nil {
		return err
	}
	if found {
		if existingTeamID == arenaTeamID {
			return tx.Commit()
		}
		return ErrArenaTeamAlreadyInTeam
	}

	memberCount, err := arenaTeamMemberCount(ctx, tx, arenaTeamID)
	if err != nil {
		return err
	}
	if memberCount >= uint32(arenaType)*2 {
		return ErrArenaTeamRosterFull
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO arena_team_member (arenaTeamId, guid, personalRating)
VALUES (?, ?, ?)`, arenaTeamID, playerGUID, personalRating); err != nil {
		return err
	}

	if err = removeArenaPetitionsAndSigns(ctx, tx, playerGUID, arenaType); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *ArenaTeamsMYSQL) RemoveMember(ctx context.Context, realmID uint32, arenaTeamID uint32, playerGUID uint64, actorGUID uint64) error {
	db, err := r.dbForRealm(realmID)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	header, err := arenaTeamHeaderForUpdate(ctx, tx, arenaTeamID)
	if err != nil {
		return err
	}
	if actorGUID != 0 {
		if playerGUID == header.captainGUID {
			return ErrArenaTeamLeaderLeave
		}
		if actorGUID != playerGUID && actorGUID != header.captainGUID {
			return ErrArenaTeamPermission
		}
	}

	res, err := tx.ExecContext(ctx, "DELETE FROM arena_team_member WHERE arenaTeamId = ? AND guid = ?", arenaTeamID, playerGUID)
	if err != nil {
		return err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows == 0 {
		return ErrArenaTeamMemberMismatch
	}

	return tx.Commit()
}

func (r *ArenaTeamsMYSQL) Disband(ctx context.Context, realmID uint32, arenaTeamID uint32, actorGUID uint64) error {
	db, err := r.dbForRealm(realmID)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	header, err := arenaTeamHeaderForUpdate(ctx, tx, arenaTeamID)
	if err != nil {
		return err
	}
	if actorGUID != 0 && actorGUID != header.captainGUID {
		return ErrArenaTeamPermission
	}

	if _, err = tx.ExecContext(ctx, "DELETE FROM arena_team_member WHERE arenaTeamId = ?", arenaTeamID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM arena_team WHERE arenaTeamId = ?", arenaTeamID); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *ArenaTeamsMYSQL) SetCaptain(ctx context.Context, realmID uint32, arenaTeamID uint32, captainGUID uint64, actorGUID uint64) error {
	db, err := r.dbForRealm(realmID)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	header, err := arenaTeamHeaderForUpdate(ctx, tx, arenaTeamID)
	if err != nil {
		return err
	}
	if actorGUID != 0 && actorGUID != header.captainGUID {
		return ErrArenaTeamPermission
	}

	isMember, err := arenaTeamHasMember(ctx, tx, arenaTeamID, captainGUID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrArenaTeamMemberMismatch
	}

	if _, err = tx.ExecContext(ctx, "UPDATE arena_team SET captainGuid = ? WHERE arenaTeamId = ?", captainGUID, arenaTeamID); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *ArenaTeamsMYSQL) SetName(ctx context.Context, realmID uint32, arenaTeamID uint32, name string, actorGUID uint64) error {
	if !isValidArenaTeamName(name) {
		return ErrArenaTeamInvalidName
	}

	db, err := r.dbForRealm(realmID)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	header, err := arenaTeamHeaderForUpdate(ctx, tx, arenaTeamID)
	if err != nil {
		return err
	}
	if actorGUID != 0 && actorGUID != header.captainGUID {
		return ErrArenaTeamPermission
	}

	var existingTeamID uint32
	err = tx.QueryRowContext(ctx, "SELECT arenaTeamId FROM arena_team WHERE name = ? LIMIT 1", name).Scan(&existingTeamID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && existingTeamID != arenaTeamID {
		return ErrArenaTeamNameExists
	}

	if _, err = tx.ExecContext(ctx, "UPDATE arena_team SET name = ? WHERE arenaTeamId = ?", name, arenaTeamID); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *ArenaTeamsMYSQL) SaveStats(ctx context.Context, request ArenaTeamSaveStatsRequest) error {
	db, err := r.dbForRealm(request.RealmID)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	arenaType, err := arenaTeamTypeForUpdate(ctx, tx, request.ArenaTeamID)
	if err != nil {
		return err
	}
	rank, err := arenaTeamRankForRating(ctx, tx, request.ArenaTeamID, arenaType, request.Rating)
	if err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, `
UPDATE arena_team
SET rating = ?, weekGames = ?, weekWins = ?, seasonGames = ?, seasonWins = ?, `+"`rank`"+` = ?
WHERE arenaTeamId = ?`,
		request.Rating,
		request.WeekGames,
		request.WeekWins,
		request.SeasonGames,
		request.SeasonWins,
		rank,
		request.ArenaTeamID,
	); err != nil {
		return err
	}

	for _, member := range request.Members {
		isMember, err := arenaTeamHasMember(ctx, tx, request.ArenaTeamID, member.PlayerGUID)
		if err != nil {
			return err
		}
		if !isMember {
			return ErrArenaTeamMemberMismatch
		}

		if _, err = tx.ExecContext(ctx, `
UPDATE arena_team_member
SET personalRating = ?, weekGames = ?, weekWins = ?, seasonGames = ?, seasonWins = ?
WHERE arenaTeamId = ? AND guid = ?`,
			member.PersonalRating,
			member.WeekGames,
			member.WeekWins,
			member.SeasonGames,
			member.SeasonWins,
			request.ArenaTeamID,
			member.PlayerGUID,
		); err != nil {
			return err
		}

		if member.SaveArenaStats {
			if _, err = tx.ExecContext(ctx, `
REPLACE INTO character_arena_stats (guid, slot, matchMakerRating, maxMMR)
VALUES (?, ?, ?, ?)`,
				member.PlayerGUID,
				request.Slot,
				member.MatchmakerRating,
				member.MaxMMR,
			); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (r *ArenaTeamsMYSQL) DeleteAll(ctx context.Context, realmID uint32) error {
	db, err := r.dbForRealm(realmID)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err = tx.ExecContext(ctx, "DELETE FROM arena_team_member"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM arena_team"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM character_arena_stats"); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *ArenaTeamsMYSQL) ValidateCharacterDelete(ctx context.Context, realmID uint32, playerGUID uint64) error {
	db, err := r.dbForRealm(realmID)
	if err != nil {
		return err
	}

	var arenaTeamID uint32
	err = db.QueryRowContext(ctx, "SELECT arenaTeamId FROM arena_team WHERE captainGuid = ? LIMIT 1", playerGUID).Scan(&arenaTeamID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	return ErrArenaTeamLeaderLeave
}

func (r *ArenaTeamsMYSQL) RemovePlayerFromTeams(ctx context.Context, realmID uint32, playerGUID uint64) error {
	db, err := r.dbForRealm(realmID)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var arenaTeamID uint32
	err = tx.QueryRowContext(ctx, "SELECT arenaTeamId FROM arena_team WHERE captainGuid = ? LIMIT 1 FOR UPDATE", playerGUID).Scan(&arenaTeamID)
	if err == nil {
		return ErrArenaTeamLeaderLeave
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if _, err = tx.ExecContext(ctx, "DELETE FROM arena_team_member WHERE guid = ?", playerGUID); err != nil {
		return err
	}

	return tx.Commit()
}

type arenaPetition struct {
	petitionID   uint32
	ownerGUID    uint64
	petitionGUID uint64
	name         string
	petitionType uint8
}

type arenaPetitionSignature struct {
	playerGUID    uint64
	playerAccount uint32
}

func arenaMemberGUIDsForCreate(captainGUID uint64, signatures []arenaPetitionSignature, arenaType uint8) []uint64 {
	memberGUIDs := make([]uint64, 0, len(signatures)+1)
	memberGUIDs = append(memberGUIDs, captainGUID)

	maxMembers := int(arenaType) * 2
	for _, signature := range signatures {
		if len(memberGUIDs) >= maxMembers {
			break
		}
		memberGUIDs = append(memberGUIDs, signature.playerGUID)
	}

	return memberGUIDs
}

func arenaPetitionForUpdate(ctx context.Context, tx *sql.Tx, petitionGuidLow uint64) (*arenaPetition, error) {
	row := tx.QueryRowContext(ctx, `
SELECT petition_id, ownerguid, petitionguid, name, type
FROM petition
WHERE petitionguid = ?
FOR UPDATE`, petitionGuidLow)

	petition := arenaPetition{}
	err := row.Scan(&petition.petitionID, &petition.ownerGUID, &petition.petitionGUID, &petition.name, &petition.petitionType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &petition, nil
}

func arenaPetitionByGUID(ctx context.Context, db *sql.DB, petitionGuidLow uint64) (*arenaPetition, error) {
	row := db.QueryRowContext(ctx, `
SELECT petition_id, ownerguid, petitionguid, name, type
FROM petition
WHERE petitionguid = ?`, petitionGuidLow)

	petition := arenaPetition{}
	err := row.Scan(&petition.petitionID, &petition.ownerGUID, &petition.petitionGUID, &petition.name, &petition.petitionType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &petition, nil
}

func arenaPetitionSignatureCount(ctx context.Context, db *sql.DB, petitionID uint32) (uint32, error) {
	var count uint32
	err := db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT player_account) FROM petition_sign WHERE petition_id = ?", petitionID).Scan(&count)
	return count, err
}

func arenaTeamNameExists(ctx context.Context, tx *sql.Tx, name string) (bool, error) {
	var exists uint8
	err := tx.QueryRowContext(ctx, "SELECT 1 FROM arena_team WHERE name = ? LIMIT 1", name).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return exists == 1, nil
}

func arenaPetitionSignatures(ctx context.Context, tx *sql.Tx, petitionID uint32) ([]arenaPetitionSignature, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT playerguid, player_account
FROM petition_sign
WHERE petition_id = ?
ORDER BY playerguid ASC`, petitionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signatures []arenaPetitionSignature
	seenAccounts := map[uint32]struct{}{}
	for rows.Next() {
		var signature arenaPetitionSignature
		if err = rows.Scan(&signature.playerGUID, &signature.playerAccount); err != nil {
			return nil, err
		}
		if _, ok := seenAccounts[signature.playerAccount]; ok {
			continue
		}
		seenAccounts[signature.playerAccount] = struct{}{}
		signatures = append(signatures, signature)
	}

	return signatures, rows.Err()
}

func ensureArenaMembersAvailable(ctx context.Context, tx *sql.Tx, memberGUIDs []uint64, arenaType uint8) error {
	for _, memberGUID := range memberGUIDs {
		var existing uint8
		err := tx.QueryRowContext(ctx, `
SELECT 1
FROM arena_team_member AS atm
INNER JOIN arena_team AS at ON at.arenaTeamId = atm.arenaTeamId
WHERE atm.guid = ? AND at.type = ?
LIMIT 1`, memberGUID, arenaType).Scan(&existing)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return err
		}
		return ErrArenaTeamAlreadyInTeam
	}
	return nil
}

func arenaTeamTypeForUpdate(ctx context.Context, tx *sql.Tx, arenaTeamID uint32) (uint8, error) {
	header, err := arenaTeamHeaderForUpdate(ctx, tx, arenaTeamID)
	if err != nil {
		return 0, err
	}
	return header.arenaType, nil
}

func arenaTeamRankForRating(ctx context.Context, tx *sql.Tx, arenaTeamID uint32, arenaType uint8, rating uint32) (uint32, error) {
	var higherRated uint32
	err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM arena_team
WHERE type = ? AND arenaTeamId <> ? AND rating > ?`, arenaType, arenaTeamID, rating).Scan(&higherRated)
	if err != nil {
		return 0, err
	}
	return higherRated + 1, nil
}

type arenaTeamHeader struct {
	arenaType   uint8
	captainGUID uint64
	name        string
}

func arenaTeamHeaderForUpdate(ctx context.Context, tx *sql.Tx, arenaTeamID uint32) (*arenaTeamHeader, error) {
	header := arenaTeamHeader{}
	err := tx.QueryRowContext(ctx, `
SELECT type, captainGuid, name
FROM arena_team
WHERE arenaTeamId = ?
FOR UPDATE`, arenaTeamID).Scan(&header.arenaType, &header.captainGUID, &header.name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrArenaTeamNotFound
		}
		return nil, err
	}
	if _, ok := arenaSlotByType(header.arenaType); !ok {
		return nil, ErrArenaTeamInvalidType
	}
	return &header, nil
}

func arenaMemberTeamForType(ctx context.Context, tx *sql.Tx, playerGUID uint64, arenaType uint8) (uint32, bool, error) {
	var arenaTeamID uint32
	err := tx.QueryRowContext(ctx, `
SELECT atm.arenaTeamId
FROM arena_team_member AS atm
INNER JOIN arena_team AS at ON at.arenaTeamId = atm.arenaTeamId
WHERE atm.guid = ? AND at.type = ?
LIMIT 1
FOR UPDATE`, playerGUID, arenaType).Scan(&arenaTeamID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return arenaTeamID, true, nil
}

func arenaTeamMemberCount(ctx context.Context, tx *sql.Tx, arenaTeamID uint32) (uint32, error) {
	var count uint32
	err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM arena_team_member WHERE arenaTeamId = ?", arenaTeamID).Scan(&count)
	return count, err
}

func arenaTeamHasMember(ctx context.Context, tx *sql.Tx, arenaTeamID uint32, playerGUID uint64) (bool, error) {
	var exists uint8
	err := tx.QueryRowContext(ctx, "SELECT 1 FROM arena_team_member WHERE arenaTeamId = ? AND guid = ? LIMIT 1", arenaTeamID, playerGUID).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return exists == 1, nil
}

func nextArenaTeamID(ctx context.Context, tx *sql.Tx) (uint32, error) {
	var nextID uint32
	err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(arenaTeamId), 0) + 1 FROM arena_team").Scan(&nextID)
	return nextID, err
}

func isValidArenaTeamName(name string) bool {
	if !utf8.ValidString(name) {
		return false
	}

	runes := []rune(name)
	if len(runes) < minArenaTeamNameRunes || len(runes) > maxArenaTeamNameRunes {
		return false
	}

	family := arenaTeamNameFamilyNone
	for _, r := range runes {
		if isArenaTeamNameNumericOrSpace(r) {
			continue
		}

		current := arenaTeamNameFamilyForRune(r)
		if current == arenaTeamNameFamilyNone {
			return false
		}
		if family != arenaTeamNameFamilyNone && family != current {
			return false
		}
		family = current
	}

	return true
}

type arenaTeamNameFamily uint8

const (
	arenaTeamNameFamilyNone arenaTeamNameFamily = iota
	arenaTeamNameFamilyLatin
	arenaTeamNameFamilyCyrillic
	arenaTeamNameFamilyEastAsian
)

func arenaTeamNameFamilyForRune(r rune) arenaTeamNameFamily {
	switch {
	case isArenaTeamNameExtendedLatin(r):
		return arenaTeamNameFamilyLatin
	case isArenaTeamNameCyrillic(r):
		return arenaTeamNameFamilyCyrillic
	case isArenaTeamNameEastAsian(r):
		return arenaTeamNameFamilyEastAsian
	default:
		return arenaTeamNameFamilyNone
	}
}

func isArenaTeamNameNumericOrSpace(r rune) bool {
	return (r >= '0' && r <= '9') || r == ' '
}

func isArenaTeamNameBasicLatin(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isArenaTeamNameExtendedLatin(r rune) bool {
	return isArenaTeamNameBasicLatin(r) ||
		(r >= 0x00C0 && r <= 0x00D6) ||
		(r >= 0x00D8 && r <= 0x00DE) ||
		r == 0x00DF ||
		(r >= 0x00E0 && r <= 0x00F6) ||
		(r >= 0x00F8 && r <= 0x00FE) ||
		(r >= 0x0100 && r <= 0x012F) ||
		r == 0x1E9E
}

func isArenaTeamNameCyrillic(r rune) bool {
	return (r >= 0x0410 && r <= 0x044F) ||
		r == 0x0401 ||
		r == 0x0451
}

func isArenaTeamNameEastAsian(r rune) bool {
	return (r >= 0x1100 && r <= 0x11F9) ||
		(r >= 0x3041 && r <= 0x30FF) ||
		(r >= 0x3131 && r <= 0x318E) ||
		(r >= 0x31F0 && r <= 0x31FF) ||
		(r >= 0x3400 && r <= 0x4DB5) ||
		(r >= 0x4E00 && r <= 0x9FC3) ||
		(r >= 0xAC00 && r <= 0xD7A3) ||
		(r >= 0xFF01 && r <= 0xFFEE)
}

func removeArenaPetitionsAndSigns(ctx context.Context, tx *sql.Tx, memberGUID uint64, arenaType uint8) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM petition_sign WHERE playerguid = ? AND type = ?", memberGUID, arenaType); err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, "SELECT petition_id FROM petition WHERE ownerguid = ? AND type = ?", memberGUID, arenaType)
	if err != nil {
		return err
	}

	var petitionIDs []uint32
	for rows.Next() {
		var petitionID uint32
		if err = rows.Scan(&petitionID); err != nil {
			_ = rows.Close()
			return err
		}
		petitionIDs = append(petitionIDs, petitionID)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}

	for _, petitionID := range petitionIDs {
		if _, err = tx.ExecContext(ctx, "DELETE FROM petition_sign WHERE petition_id = ?", petitionID); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM petition WHERE ownerguid = ? AND type = ?", memberGUID, arenaType)
	return err
}
