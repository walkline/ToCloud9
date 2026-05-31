package proxy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/mysqlreverseproxy/sqlparser"
	wowarena "github.com/walkline/ToCloud9/shared/wow/arena"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

// StmtWithParsedDataContext holds data related to prepared statements and GUIDs for each statement context.
type StmtWithParsedDataContext struct {
	Stmts           map[uint32]DBStatement    // Mapped by realm ID
	GuidFinder      *sqlparser.CharGUIDFinder // Helper to extract GUIDs from queries
	IsDummy         bool                      // Flag indicating if it's a dummy statement
	RunOnEveryRealm bool                      // Replicated instance-state writes must stay present in every home realm DB.
}

// TransactionState represents the state of a transaction.
type TransactionState uint8

const (
	TransactionNone       TransactionState = iota // No transaction in progress
	TransactionRequested                          // Transaction has been requested (but not started)
	TransactionInProgress                         // Transaction is in progress
)

const (
	StartTransaction    = "START TRANSACTION" // Query to start a transaction
	CommitTransaction   = "COMMIT"            // Query to commit a transaction
	RollbackTransaction = "ROLLBACK"          // Query to rollback a transaction
	DummyPreparedStmt   = "SELECT ? AS no_op" // Dummy query used for no-op statements
)

// TC9Proxy represents the proxy for handling database connections and transaction state.
type TC9Proxy struct {
	connectionByRealm    map[uint32]DBConnection // Connection mapping by realm ID
	clientID             int                     // Client ID
	parser               *parser.Parser          // SQL parser
	transactionState     TransactionState        // Current transaction state
	transactionRealmID   uint32                  // Realm ID for the current transaction
	transactionAllRealms bool                    // Transaction was opened on every realm connection.
}

// NewTC9Proxy creates a new instance of TC9Proxy.
func NewTC9Proxy(clientID int, connections map[uint32]DBConnection, parser *parser.Parser) *TC9Proxy {
	return &TC9Proxy{
		connectionByRealm: connections,
		clientID:          clientID,
		parser:            parser,
		transactionState:  TransactionNone,
	}
}

// ConnByRealm retrieves the connection for a given realm ID, or a fallback connection.
func (p *TC9Proxy) ConnByRealm(realmID uint32) DBConnection {
	if p.connectionByRealm[realmID] == nil {
		// If no connection is found for the realm, return the first available
		// connection deterministically. Map iteration order is random in Go.
		realms := make([]int, 0, len(p.connectionByRealm))
		for realm := range p.connectionByRealm {
			realms = append(realms, int(realm))
		}
		sort.Ints(realms)
		for _, realm := range realms {
			if conn := p.connectionByRealm[uint32(realm)]; conn != nil {
				return conn
			}
		}
	}
	return p.connectionByRealm[realmID]
}

// RunOnEveryConn runs a function on every database connection.
func (p *TC9Proxy) RunOnEveryConn(f func(db DBConnection)) {
	for _, conn := range p.connectionByRealm {
		f(conn)
	}
}

// UseDB sets the database to use (does nothing here).
func (p *TC9Proxy) UseDB(string) error {
	return nil
}

// HandleQuery processes a query, handles transactions, and executes the query.
func (p *TC9Proxy) HandleQuery(query string) (*mysql.Result, error) {
	query = strings.TrimSpace(query)
	normalizedQuery := strings.ToUpper(query)

	// Default connection (realm 0)
	conn := p.ConnByRealm(0)
	switch normalizedQuery {
	case StartTransaction:
		p.transactionState = TransactionRequested // Start transaction flag
		p.transactionAllRealms = false
		return nil, nil
	case CommitTransaction, RollbackTransaction:
		defer func() {
			p.transactionState = TransactionNone
			p.transactionAllRealms = false
			p.transactionRealmID = 0
		}()
		if p.transactionAllRealms {
			return p.executeQueryOnEveryConn(normalizedQuery)
		}
		conn = p.ConnByRealm(p.transactionRealmID) // Use realm connection for commit/rollback
	}

	if querySelectsCharacterInstance(normalizedQuery) {
		return p.selectCharacterInstancesFromEveryRealm(normalizedQuery)
	}
	if querySelectsArenaTeamMembers(normalizedQuery) {
		return p.selectArenaTeamMembersFromEveryRealm(normalizedQuery)
	}
	if querySelectsArenaTeams(normalizedQuery) {
		return p.selectArenaTeamsFromEveryRealm(normalizedQuery)
	}
	if queryShouldRunOnEveryRealm(normalizedQuery) {
		return p.executeQueryOnEveryConn(normalizedQuery)
	}

	return conn.Execute(normalizedQuery)
}

// HandleFieldList retrieves a list of fields for a given table.
func (p *TC9Proxy) HandleFieldList(table string, fieldWildcard string) ([]*mysql.Field, error) {
	return p.ConnByRealm(0).FieldList(table, fieldWildcard)
}

// HandleStmtPrepare prepares a statement by parsing the query and extracting GUIDs.
func (p *TC9Proxy) HandleStmtPrepare(query string) (int, int, interface{}, error) {
	stmtNode, err := p.parser.ParseOneStmt(query, "utf8mb4", "utf8mb4_bin")
	if err != nil {
		return 0, 0, nil, err
	}

	// Extract GUID indexes from the statement
	v := sqlparser.NewCharGUIDFinder()
	stmtNode.Accept(&v)
	v.FillInGUIDIndexes()

	// Prepare statements for each realm
	stmtsWithParsedData := &StmtWithParsedDataContext{
		GuidFinder:      &v,
		Stmts:           map[uint32]DBStatement{},
		RunOnEveryRealm: statementShouldRunOnEveryRealm(&v),
	}

	var stmt DBStatement
	for realm := range p.connectionByRealm {
		stmt, err = p.connectionByRealm[realm].Prepare(query)
		if err != nil {
			return 0, 0, nil, err
		}

		stmtsWithParsedData.Stmts[realm] = stmt
	}

	if stmt == nil {
		return 0, 0, nil, fmt.Errorf("no stmt found for realm %d", p.clientID)
	}

	if query == DummyPreparedStmt {
		stmtsWithParsedData.IsDummy = true
	}

	return stmt.ParamNum(), stmt.ColumnNum(), stmtsWithParsedData, nil
}

// HandleStmtExecute executes a prepared statement with given arguments.
func (p *TC9Proxy) HandleStmtExecute(ctx interface{}, query string, args []interface{}) (*mysql.Result, error) {
	if ctx == nil {
		return nil, fmt.Errorf("stmt not found")
	}

	stmtWithParsedData := ctx.(*StmtWithParsedDataContext)
	realmID, rawGUID, realGUIDCounter := extractGUIDAndRealmID(stmtWithParsedData, args, p)

	if err := p.startTransactionIfNeeded(args, stmtWithParsedData, realmID); err != nil {
		return nil, err
	}

	if stmtWithParsedData.IsDummy {
		return nil, nil
	}
	if stmtWithParsedData.RunOnEveryRealm {
		return p.executeStmtOnEveryRealm(stmtWithParsedData, args)
	}

	// Get the correct statement for the realm
	stmt := stmtWithParsedData.Stmts[realmID]
	if stmt == nil {
		return nil, fmt.Errorf("statement not found for realm %d", realmID)
	}

	// Execute the statement
	log.Trace().
		Str("Query", query).
		Uint32("RealmID", realmID).
		Uint32("Guid", realGUIDCounter).
		Uint64("Raw GUID", rawGUID).
		Msg("ExecStmt")

	res, err := stmt.Execute(args...)
	if err != nil {
		return nil, err
	}

	updateOutputGUIDs(res, stmtWithParsedData, rawGUID)
	updateOutputArenaTeamIDs(res, stmtWithParsedData, realmID)

	return res, nil
}

// HandleStmtClose closes a prepared statement.
func (p *TC9Proxy) HandleStmtClose(context interface{}) error {
	if context == nil {
		return fmt.Errorf("stmt not found")
	}
	for _, stmt := range context.(*StmtWithParsedDataContext).Stmts {
		if err := stmt.Close(); err != nil {
			return err
		}
	}
	return nil
}

// HandleOtherCommand processes unsupported commands and returns an error.
func (p *TC9Proxy) HandleOtherCommand(cmd byte, data []byte) error {
	return mysql.NewError(
		mysql.ER_UNKNOWN_ERROR,
		fmt.Sprintf("command %d is not supported now", cmd),
	)
}

// extractGUIDAndRealmID extracts the GUID and realm ID from the prepared statement context.
func extractGUIDAndRealmID(stmtWithParsedData *StmtWithParsedDataContext, args []interface{}, p *TC9Proxy) (uint32, uint64, uint32) {
	var rawGUID uint64
	var realmID uint32
	var realGUIDCounter uint32

	// Extract GUID from the statement arguments
	if len(stmtWithParsedData.GuidFinder.InputGUIDIndexes) > 0 {
		g := args[stmtWithParsedData.GuidFinder.InputGUIDIndexes[0]]
		switch v := g.(type) {
		case uint64:
			rawGUID = v
		case uint32:
			rawGUID = uint64(v)
		}

		// Create a GUID object and extract realm ID and counter
		playerGUID := guid.New(rawGUID)
		realmID = uint32(playerGUID.GetRealmID())
		realGUIDCounter = uint32(playerGUID.GetCounter())

		// Replace the GUID in the arguments with the real counter value
		for _, i := range stmtWithParsedData.GuidFinder.InputGUIDIndexes {
			args[i] = realGUIDCounter
		}
	}

	if len(stmtWithParsedData.GuidFinder.InputArenaTeamIDIndexes) > 0 {
		var teamID uint32
		for _, i := range stmtWithParsedData.GuidFinder.InputArenaTeamIDIndexes {
			teamID = numericArgToUint32(args[i])
			teamRealmID := wowarena.TeamIDRealmID(teamID)
			if teamRealmID != 0 {
				if realmID == 0 {
					realmID = teamRealmID
				}
				args[i] = wowarena.TeamIDCounter(teamID)
			}
		}
	}

	// Default realm ID logic
	if realmID == 0 {
		if !stmtWithParsedData.GuidFinder.IsSelectStmt && p.transactionState == TransactionInProgress {
			realmID = p.transactionRealmID
		} else {
			realmID = 1 // Default to realm 1 if unknown
		}
	}

	return realmID, rawGUID, realGUIDCounter
}

func numericArgToUint32(arg interface{}) uint32 {
	switch v := arg.(type) {
	case uint64:
		return uint32(v)
	case uint32:
		return v
	case uint:
		return uint32(v)
	case uint16:
		return uint32(v)
	case uint8:
		return uint32(v)
	case int64:
		return uint32(v)
	case int32:
		return uint32(v)
	case int:
		return uint32(v)
	case int16:
		return uint32(v)
	case int8:
		return uint32(v)
	default:
		return 0
	}
}

// startTransactionIfNeeded starts a transaction if needed based on the query and state.
func (p *TC9Proxy) startTransactionIfNeeded(args []interface{}, stmtWithParsedData *StmtWithParsedDataContext, realmID uint32) error {
	// Logic to start a transaction if required (for dummy queries or transaction control)
	if stmtWithParsedData.IsDummy && p.transactionState == TransactionRequested {
		contextRealmID, err := explicitRealmContext(args)
		if err != nil {
			return err
		}
		if _, err := p.ConnByRealm(contextRealmID).Execute(StartTransaction); err != nil {
			return fmt.Errorf("failed to start transaction, err: %v", err)
		}

		p.transactionState = TransactionInProgress
		p.transactionRealmID = contextRealmID
		return nil
	}

	if p.transactionState == TransactionRequested {
		if stmtWithParsedData.RunOnEveryRealm {
			if _, err := p.executeQueryOnEveryConn(StartTransaction); err != nil {
				return fmt.Errorf("failed to start replicated transaction, err: %v", err)
			}
			p.transactionState = TransactionInProgress
			p.transactionAllRealms = true
			p.transactionRealmID = 0
			return nil
		}
		if _, err := p.ConnByRealm(realmID).Execute(StartTransaction); err != nil {
			return fmt.Errorf("failed to start transaction, err: %v", err)
		}
		p.transactionState = TransactionInProgress
		p.transactionRealmID = realmID
	}

	return nil
}

func explicitRealmContext(args []interface{}) (uint32, error) {
	if len(args) == 0 {
		return 0, fmt.Errorf("missing realmID context provider argument")
	}

	switch v := args[0].(type) {
	case uint64:
		return uint32(v), nil
	case uint32:
		return v, nil
	case uint:
		return uint32(v), nil
	case uint16:
		return uint32(v), nil
	case uint8:
		return uint32(v), nil
	case int64:
		if v < 0 {
			return 0, fmt.Errorf("invalid negative realmID context provider argument: %d", v)
		}
		return uint32(v), nil
	case int32:
		if v < 0 {
			return 0, fmt.Errorf("invalid negative realmID context provider argument: %d", v)
		}
		return uint32(v), nil
	case int:
		if v < 0 {
			return 0, fmt.Errorf("invalid negative realmID context provider argument: %d", v)
		}
		return uint32(v), nil
	case int16:
		if v < 0 {
			return 0, fmt.Errorf("invalid negative realmID context provider argument: %d", v)
		}
		return uint32(v), nil
	case int8:
		if v < 0 {
			return 0, fmt.Errorf("invalid negative realmID context provider argument: %d", v)
		}
		return uint32(v), nil
	default:
		return 0, fmt.Errorf("invalid realmID context provider argument type: %T", v)
	}
}

func (p *TC9Proxy) executeQueryOnEveryConn(query string) (*mysql.Result, error) {
	var first *mysql.Result
	for _, realm := range p.sortedRealms() {
		res, err := p.connectionByRealm[realm].Execute(query)
		if err != nil {
			return nil, err
		}
		if first == nil {
			first = res
		}
	}
	return first, nil
}

func (p *TC9Proxy) executeStmtOnEveryRealm(stmtWithParsedData *StmtWithParsedDataContext, args []interface{}) (*mysql.Result, error) {
	var first *mysql.Result
	for _, realm := range p.sortedRealms() {
		stmt := stmtWithParsedData.Stmts[realm]
		if stmt == nil {
			return nil, fmt.Errorf("statement not found for realm %d", realm)
		}

		res, err := stmt.Execute(args...)
		if err != nil {
			return nil, err
		}
		if first == nil {
			first = res
		}
	}
	return first, nil
}

func (p *TC9Proxy) sortedRealms() []uint32 {
	realms := make([]int, 0, len(p.connectionByRealm))
	for realm := range p.connectionByRealm {
		realms = append(realms, int(realm))
	}
	sort.Ints(realms)

	result := make([]uint32, 0, len(realms))
	for _, realm := range realms {
		result = append(result, uint32(realm))
	}
	return result
}

func (p *TC9Proxy) selectCharacterInstancesFromEveryRealm(query string) (*mysql.Result, error) {
	var merged *mysql.Result
	for _, realm := range p.sortedRealms() {
		res, err := p.connectionByRealm[realm].Execute(query)
		if err != nil {
			return nil, err
		}
		if res == nil || res.Resultset == nil {
			continue
		}
		rewriteCharacterInstanceGUIDs(res, realm)
		if merged == nil {
			merged = res
			continue
		}
		merged.Values = append(merged.Values, res.Values...)
	}
	if merged == nil {
		return &mysql.Result{}, nil
	}
	return merged, nil
}

func (p *TC9Proxy) selectArenaTeamsFromEveryRealm(query string) (*mysql.Result, error) {
	return p.selectArenaRowsFromEveryRealm(query, rewriteArenaTeamRows)
}

func (p *TC9Proxy) selectArenaTeamMembersFromEveryRealm(query string) (*mysql.Result, error) {
	return p.selectArenaRowsFromEveryRealm(query, rewriteArenaTeamMemberRows)
}

func (p *TC9Proxy) selectArenaRowsFromEveryRealm(query string, rewrite func(*mysql.Result, uint32)) (*mysql.Result, error) {
	var merged *mysql.Result
	for _, realm := range p.sortedRealms() {
		res, err := p.connectionByRealm[realm].Execute(query)
		if err != nil {
			return nil, err
		}
		if res == nil || res.Resultset == nil {
			continue
		}
		rewrite(res, realm)
		if merged == nil {
			merged = res
			continue
		}
		merged.Values = append(merged.Values, res.Values...)
	}
	if merged == nil {
		return &mysql.Result{}, nil
	}
	return merged, nil
}

func rewriteCharacterInstanceGUIDs(res *mysql.Result, realm uint32) {
	if res == nil || res.Resultset == nil || realm == 0 {
		return
	}

	guidColumn, ok := res.FieldNames["guid"]
	if !ok {
		guidColumn, ok = res.FieldNames["GUID"]
		if !ok {
			return
		}
	}

	for row := range res.Values {
		if guidColumn >= len(res.Values[row]) {
			continue
		}
		low := res.Values[row][guidColumn].AsUint64()
		if low == 0 || low>>32 != 0 {
			continue
		}
		raw := guid.NewCrossrealmPlayerGUID(uint16(realm), guid.LowType(low)).GetRawValue()
		res.Values[row][guidColumn] = mysql.NewFieldValue(mysql.FieldValueTypeUnsigned, raw, []byte(fmt.Sprintf("%d", raw)))
	}
}

func rewriteArenaTeamRows(res *mysql.Result, realm uint32) {
	if res == nil || res.Resultset == nil || realm == 0 {
		return
	}

	rewriteArenaTeamIDColumn(res, realm, "arenaTeamId")
	rewritePlayerGUIDColumn(res, realm, "captainGuid")
}

func rewriteArenaTeamMemberRows(res *mysql.Result, realm uint32) {
	if res == nil || res.Resultset == nil || realm == 0 {
		return
	}

	rewriteArenaTeamIDColumn(res, realm, "arenaTeamId")
	rewritePlayerGUIDColumn(res, realm, "guid")
}

func rewriteArenaTeamIDColumn(res *mysql.Result, realm uint32, column string) {
	columnIndex, ok := resultColumnIndex(res, column)
	if !ok {
		return
	}

	for row := range res.Values {
		if columnIndex >= len(res.Values[row]) {
			continue
		}
		low := uint32(res.Values[row][columnIndex].AsUint64())
		if low == 0 || wowarena.TeamIDRealmID(low) != 0 {
			continue
		}
		raw := wowarena.NewCrossrealmTeamID(uint16(realm), low)
		res.Values[row][columnIndex] = mysql.NewFieldValue(mysql.FieldValueTypeUnsigned, uint64(raw), []byte(fmt.Sprintf("%d", raw)))
	}
}

func rewritePlayerGUIDColumn(res *mysql.Result, realm uint32, column string) {
	columnIndex, ok := resultColumnIndex(res, column)
	if !ok {
		return
	}

	for row := range res.Values {
		if columnIndex >= len(res.Values[row]) {
			continue
		}
		low := res.Values[row][columnIndex].AsUint64()
		if low == 0 || low>>32 != 0 {
			continue
		}
		raw := guid.NewCrossrealmPlayerGUID(uint16(realm), guid.LowType(low)).GetRawValue()
		res.Values[row][columnIndex] = mysql.NewFieldValue(mysql.FieldValueTypeUnsigned, raw, []byte(fmt.Sprintf("%d", raw)))
	}
}

func resultColumnIndex(res *mysql.Result, column string) (int, bool) {
	if res == nil || res.Resultset == nil {
		return 0, false
	}
	for fieldName, index := range res.FieldNames {
		if strings.EqualFold(fieldName, column) {
			return index, true
		}
	}
	return 0, false
}

func querySelectsCharacterInstance(query string) bool {
	return isSelectQuery(query) && strings.Contains(query, "CHARACTER_INSTANCE")
}

func querySelectsArenaTeams(query string) bool {
	return isSelectQuery(query) && strings.Contains(query, "FROM ARENA_TEAM") && !strings.Contains(query, "FROM ARENA_TEAM_MEMBER")
}

func querySelectsArenaTeamMembers(query string) bool {
	return isSelectQuery(query) && strings.Contains(query, "FROM ARENA_TEAM_MEMBER")
}

func queryShouldRunOnEveryRealm(query string) bool {
	if isSelectQuery(query) {
		return false
	}
	if queryTouchesArenaGlobalTable(query) {
		return true
	}
	if queryTouchesReplicatedInstanceStateTable(query) {
		return true
	}

	// Direct character_instance writes in AzerothCore are startup/global cleanup
	// queries. Player-specific writes use prepared statements so the proxy can
	// route by realm-scoped GUID instead.
	return strings.Contains(query, "CHARACTER_INSTANCE")
}

func statementShouldRunOnEveryRealm(v *sqlparser.CharGUIDFinder) bool {
	if v == nil || v.IsSelectStmt {
		return false
	}
	if v.TouchesReplicatedInstanceStateTable() {
		return true
	}
	return v.TouchesTable("character_instance") && len(v.InputGUIDIndexes) == 0
}

func isSelectQuery(query string) bool {
	return strings.HasPrefix(strings.TrimSpace(query), "SELECT")
}

func queryTouchesReplicatedInstanceStateTable(query string) bool {
	for _, pattern := range []string{
		"CREATURE_RESPAWN",
		"GAMEOBJECT_RESPAWN",
		"INSTANCE_RESET",
		"INSTANCE_SAVED_GO_STATE_DATA",
		" FROM INSTANCE",
		" INTO INSTANCE",
		"INSERT INTO INSTANCE",
		"UPDATE INSTANCE",
		"DELETE FROM INSTANCE",
		" JOIN INSTANCE",
	} {
		if strings.Contains(query, pattern) {
			return true
		}
	}
	return false
}

func queryTouchesArenaGlobalTable(query string) bool {
	for _, pattern := range []string{
		"DELETE FROM ARENA_TEAM_MEMBER WHERE ARENATEAMID NOT IN",
		"DELETE FROM ARENA_TEAM_MEMBER",
		"DELETE FROM ARENA_TEAM",
		"DELETE FROM CHARACTER_ARENA_STATS",
		"TRUNCATE TABLE ARENA_TEAM_MEMBER",
		"TRUNCATE TABLE ARENA_TEAM",
		"TRUNCATE TABLE CHARACTER_ARENA_STATS",
	} {
		if strings.Contains(query, pattern) {
			return true
		}
	}
	return false
}

// updateOutputGUIDs updates the output GUIDs in the query result with crossrealm guid.
func updateOutputGUIDs(res *mysql.Result, stmtWithParsedData *StmtWithParsedDataContext, rawGUID uint64) {
	if rawGUID > 0 && len(stmtWithParsedData.GuidFinder.OutputGUIDIndexes) > 0 {
		for _, column := range stmtWithParsedData.GuidFinder.OutputGUIDIndexes {
			for row := range res.Values {
				res.Values[row][column] = mysql.NewFieldValue(mysql.FieldValueTypeUnsigned, rawGUID, []byte(fmt.Sprintf("%d", rawGUID)))
			}
		}
	}
}

func updateOutputArenaTeamIDs(res *mysql.Result, stmtWithParsedData *StmtWithParsedDataContext, realmID uint32) {
	if res == nil || stmtWithParsedData == nil || stmtWithParsedData.GuidFinder == nil || realmID == 0 || len(stmtWithParsedData.GuidFinder.OutputArenaTeamIDIndexes) == 0 {
		return
	}

	for _, column := range stmtWithParsedData.GuidFinder.OutputArenaTeamIDIndexes {
		for row := range res.Values {
			if column >= len(res.Values[row]) {
				continue
			}
			low := uint32(res.Values[row][column].AsUint64())
			if low == 0 || wowarena.TeamIDRealmID(low) != 0 {
				continue
			}
			raw := wowarena.NewCrossrealmTeamID(uint16(realmID), low)
			res.Values[row][column] = mysql.NewFieldValue(mysql.FieldValueTypeUnsigned, uint64(raw), []byte(fmt.Sprintf("%d", raw)))
		}
	}
}

// EmptyReplicationHandler is a no-op handler for replication commands.
type EmptyReplicationHandler struct{ TC9Proxy }

// HandleRegisterSlave returns an error for unsupported slave registration.
func (h *EmptyReplicationHandler) HandleRegisterSlave([]byte) error {
	return fmt.Errorf("not supported now")
}

// HandleBinlogDump returns an error for unsupported binlog dump.
func (h *EmptyReplicationHandler) HandleBinlogDump(mysql.Position) (*replication.BinlogStreamer, error) {
	return nil, fmt.Errorf("not supported now")
}

// HandleBinlogDumpGTID returns an error for unsupported binlog dump with GTID.
func (h *EmptyReplicationHandler) HandleBinlogDumpGTID(*mysql.MysqlGTIDSet) (*replication.BinlogStreamer, error) {
	return nil, fmt.Errorf("not supported now")
}
