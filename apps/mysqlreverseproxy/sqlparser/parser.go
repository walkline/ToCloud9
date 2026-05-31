package sqlparser

import (
	"strings"

	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/test_driver"
)

var charGuidColumnToTableMap = map[string]string{
	"characters":                     "guid",
	"character_instance":             "guid",
	"character_inventory":            "guid",
	"character_queststatus":          "guid",
	"mail_items":                     "receiver",
	"mail":                           "receiver",
	"character_reputation":           "guid",
	"character_queststatus_daily":    "guid",
	"character_homebind":             "guid",
	"character_social":               "guid",
	"character_spell_cooldown":       "guid",
	"character_achievement":          "guid",
	"character_achievement_progress": "guid",
	"character_equipmentsets":        "guid",
	"character_entry_point":          "guid",
	"character_glyphs":               "guid",
	"character_talent":               "guid",
	"character_account_data":         "guid",
	"character_skills":               "guid",
	"character_spell":                "guid",
	"character_action":               "guid",
	"character_queststatus_weekly":   "guid",
	"character_battleground_random":  "guid",
	"character_banned":               "guid",
	"character_queststatus_rewarded": "guid",
	"character_queststatus_seasonal": "guid",
	"character_queststatus_monthly":  "guid",
	"character_brew_of_the_month":    "guid",
	"corpse":                         "guid",
	"character_settings":             "guid",
	"character_pet":                  "owner",
	"character_aura":                 "guid",
	"item_instance":                  "owner_guid",
	"battleground_deserters":         "guid",
	"arena_team":                     "captainGuid",
	"arena_team_member":              "guid",
	"character_arena_stats":          "guid",
}

var replicatedInstanceStateTables = map[string]struct{}{
	"creature_respawn":             {},
	"gameobject_respawn":           {},
	"instance":                     {},
	"instance_reset":               {},
	"instance_saved_go_state_data": {},
}

var arenaTeamIDTables = map[string]struct{}{
	"arena_team":        {},
	"arena_team_member": {},
}

type pairOperation struct {
	column         string
	paramPresented bool
}

type CharGUIDFinder struct {
	InputGUIDIndexes         []int
	OutputGUIDIndexes        []int
	InputArenaTeamIDIndexes  []int
	OutputArenaTeamIDIndexes []int
	IsSelectStmt             bool
	prefix                   string
	tableExpSource           string
	tableNames               []string
	tableNameShortcuts       map[string]string
	binOperation             *pairOperation
	assignOperation          *pairOperation
	isInFieldList            bool
	isInsert                 bool
	insertColumns            []string
	inputParams              []string
	outputParams             []string
}

func NewCharGUIDFinder() CharGUIDFinder {
	return CharGUIDFinder{
		tableNameShortcuts: map[string]string{},
	}
}

func (v *CharGUIDFinder) Enter(in ast.Node) (ast.Node, bool) {
	v.prefix += "--"
	switch node := in.(type) {
	case *ast.InsertStmt:
		v.isInsert = true
	case *ast.SelectStmt:
		v.IsSelectStmt = true
	case *ast.TableSource:
		v.tableExpSource = strings.ToLower(node.AsName.String())
	case *ast.TableName:
		tableName := strings.ToLower(node.Name.String())
		if v.tableExpSource != "" {
			v.tableNameShortcuts[v.tableExpSource] = tableName
			v.tableExpSource = ""
		}
		v.tableNames = append(v.tableNames, tableName)
	case *ast.BinaryOperationExpr:
		v.binOperation = &pairOperation{}
	case *ast.Assignment:
		v.assignOperation = &pairOperation{}
	case *ast.FieldList:
		v.isInFieldList = true
	case *ast.ColumnNameExpr:
		v.handleColumnNameExpr(node)
	case *ast.ColumnName:
		v.handleColumnName(node)
	case *test_driver.ParamMarkerExpr:
		v.handleParamMarkerExpr()
	}
	return in, false
}

func (v *CharGUIDFinder) Leave(in ast.Node) (ast.Node, bool) {
	v.prefix = v.prefix[:len(v.prefix)-2]
	switch in.(type) {
	case *ast.BinaryOperationExpr:
		v.binOperation = nil
	case *ast.Assignment:
		v.assignOperation = nil
	case *ast.FieldList:
		v.isInFieldList = false
	}
	return in, true
}

func (v *CharGUIDFinder) handleColumnNameExpr(node *ast.ColumnNameExpr) {
	if v.binOperation != nil {
		if v.binOperation.paramPresented {
			v.inputParams[len(v.inputParams)-1] = node.Name.String()
		} else {
			v.binOperation.column = node.Name.String()
		}
	} else if v.isInFieldList {
		v.outputParams = append(v.outputParams, node.Name.String())
	}
}

func (v *CharGUIDFinder) handleColumnName(node *ast.ColumnName) {
	if v.assignOperation != nil {
		if v.assignOperation.paramPresented {
			v.inputParams[len(v.inputParams)-1] = node.Name.String()
		} else {
			v.assignOperation.column = node.Name.String()
		}
	} else if v.isInsert {
		v.insertColumns = append(v.insertColumns, node.Name.String())
	}
}

func (v *CharGUIDFinder) handleParamMarkerExpr() {
	var column string
	if v.binOperation != nil {
		v.binOperation.paramPresented = true
		column = v.binOperation.column
	} else if v.assignOperation != nil {
		v.assignOperation.paramPresented = true
		column = v.assignOperation.column
	} else if v.isInsert && len(v.insertColumns) > len(v.inputParams) {
		column = v.insertColumns[len(v.inputParams)]
	} else {
		column = ""
	}
	v.inputParams = append(v.inputParams, column)
}

func (v *CharGUIDFinder) FillInGUIDIndexes() {
	searchColumnNames := make(map[string]struct{})
	touchesArenaTeamIDs := false
	for _, name := range v.tableNames {
		columnName := charGuidColumnToTableMap[name]
		if columnName != "" {
			searchColumnNames[columnName] = struct{}{}
		}
		if _, ok := arenaTeamIDTables[name]; ok {
			touchesArenaTeamIDs = true
		}
	}

	for column := range searchColumnNames {
		v.findGUIDIndexes(column)
	}
	if touchesArenaTeamIDs {
		v.findArenaTeamIDIndexes()
	}
}

func (v *CharGUIDFinder) TouchesReplicatedInstanceStateTable() bool {
	for _, name := range v.tableNames {
		if _, ok := replicatedInstanceStateTables[name]; ok {
			return true
		}
	}
	return false
}

func (v *CharGUIDFinder) TouchesTable(tableName string) bool {
	for _, name := range v.tableNames {
		if name == tableName {
			return true
		}
	}
	return false
}

func (v *CharGUIDFinder) findGUIDIndexes(column string) {
	for i, param := range v.inputParams {
		if v.isGUIDColumn(param, column) {
			v.InputGUIDIndexes = append(v.InputGUIDIndexes, i)
		}
	}

	for i, param := range v.outputParams {
		if v.isGUIDColumn(param, column) {
			v.OutputGUIDIndexes = append(v.OutputGUIDIndexes, i)
		}
	}
}

func (v *CharGUIDFinder) findArenaTeamIDIndexes() {
	for i, param := range v.inputParams {
		if v.isArenaTeamIDColumn(param) {
			v.InputArenaTeamIDIndexes = append(v.InputArenaTeamIDIndexes, i)
		}
	}

	for i, param := range v.outputParams {
		if v.isArenaTeamIDColumn(param) {
			v.OutputArenaTeamIDIndexes = append(v.OutputArenaTeamIDIndexes, i)
		}
	}
}

func (v *CharGUIDFinder) isGUIDColumn(param, column string) bool {
	param = strings.ToLower(param)
	column = strings.ToLower(column)
	if strings.ContainsRune(param, '.') {
		strs := strings.Split(param, ".")
		tableName := v.tableNameShortcuts[strs[0]]
		if tableName == "" {
			tableName = strs[0]
		}
		charGuidColumnName := charGuidColumnToTableMap[tableName]
		return charGuidColumnName != "" && strs[1] == charGuidColumnName
	}
	return param == column
}

func (v *CharGUIDFinder) isArenaTeamIDColumn(param string) bool {
	param = strings.ToLower(param)
	if strings.ContainsRune(param, '.') {
		strs := strings.Split(param, ".")
		tableName := v.tableNameShortcuts[strs[0]]
		if tableName == "" {
			tableName = strs[0]
		}
		if _, ok := arenaTeamIDTables[tableName]; !ok {
			return false
		}
		return strings.EqualFold(strs[1], "arenateamid")
	}
	return strings.EqualFold(param, "arenateamid")
}
