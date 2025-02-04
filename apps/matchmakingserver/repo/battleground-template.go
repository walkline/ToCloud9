package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/walkline/ToCloud9/apps/matchmakingserver/battleground"
)

type BattlegroundTemplate struct {
	TypeID                   uint32
	MapID                    uint32
	MinPlayersPerTeam        uint8
	MaxPlayersPerTeam        uint8
	MinLvl                   uint8
	MaxLvl                   uint8
	RandomBattlegroundWeight int
}

func (t *BattlegroundTemplate) GetAllBrackets() []uint8 {
	res := []uint8{}
	minLvl := t.MinLvl
	if minLvl < 10 {
		minLvl = 10
	}
	maxLvl := t.MaxLvl
	if maxLvl > 80 {
		maxLvl = 80
	}
	for i := minLvl; i < maxLvl; i += 10 {
		res = append(res, battleground.BracketIDByLevel(i))
	}
	return append(res, battleground.BracketIDByLevel(t.MaxLvl))
}

type BattlegroundTemplesRepo interface {
	GetAll(context.Context) ([]BattlegroundTemplate, error)
}

type mysqlBattlegroundTemplateRepo struct {
	worldDB *sql.DB
}

func NewMySQLBattlegroundTemplateRepo(worldDB *sql.DB) BattlegroundTemplesRepo {
	return &mysqlBattlegroundTemplateRepo{
		worldDB: worldDB,
	}
}

func (m *mysqlBattlegroundTemplateRepo) GetAll(ctx context.Context) ([]BattlegroundTemplate, error) {
	rows, err := m.worldDB.QueryContext(ctx, "SELECT ID, MinPlayersPerTeam,MaxPlayersPerTeam, MinLvl, MaxLvl, Weight FROM battleground_template")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var battlegroundTemplates []BattlegroundTemplate
	for rows.Next() {
		var battlegroundTemplate BattlegroundTemplate
		err = rows.Scan(&battlegroundTemplate.TypeID, &battlegroundTemplate.MinPlayersPerTeam, &battlegroundTemplate.MaxPlayersPerTeam, &battlegroundTemplate.MinLvl, &battlegroundTemplate.MaxLvl, &battlegroundTemplate.RandomBattlegroundWeight)
		if err != nil {
			return nil, fmt.Errorf("error scanning battleground templates: %w", err)
		}

		// DEBUG:
		// battlegroundTemplate.MinPlayersPerTeam = 1

		battlegroundTemplate.MapID = battlegroundTypeToMapID(battlegroundTemplate.TypeID)
		battlegroundTemplates = append(battlegroundTemplates, battlegroundTemplate)
	}

	return battlegroundTemplates, nil
}

// TODO: remove hardcoded values
func battlegroundTypeToMapID(typeID uint32) uint32 {
	switch battleground.QueueTypeID(typeID) {
	case battleground.QueueTypeIDAlteracValley:
		return 30
	case battleground.QueueTypeIDWarsongGulch:
		return 489
	case battleground.QueueTypeIDArathiBasin:
		return 529
	case battleground.QueueTypeIDNagrandArena:
		return 559
	case battleground.QueueTypeIDBladesEdgeArena:
		return 562
	case battleground.QueueTypeIDAllArenas:
		return 559
	case battleground.QueueTypeIDEyeOfTheStorm:
		return 566
	case battleground.QueueTypeIDRuinsOfLordaeron:
		return 572
	case battleground.QueueTypeIDStrandOfTheAncients:
		return 607
	case battleground.QueueTypeIDDalaranSewers:
		return 617
	case battleground.QueueTypeIDRingOfValor:
		return 618
	case battleground.QueueTypeIDIsleOfConquest:
		return 628
	case battleground.QueueTypeIDRandomBattleground:
		return 30
	}
	return 30
}
