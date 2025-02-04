package proxy

import (
	"fmt"
	"strings"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/mysqlreverseproxy/sqlparser"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

// StmtWithParsedDataContext holds data related to prepared statements and GUIDs for each statement context.
type StmtWithParsedDataContext struct {
	Stmts      map[uint32]DBStatement    // Mapped by realm ID
	GuidFinder *sqlparser.CharGUIDFinder // Helper to extract GUIDs from queries
	IsDummy    bool                      // Flag indicating if it's a dummy statement
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
	connectionByRealm  map[uint32]DBConnection // Connection mapping by realm ID
	clientID           int                     // Client ID
	parser             *parser.Parser          // SQL parser
	transactionState   TransactionState        // Current transaction state
	transactionRealmID uint32                  // Realm ID for the current transaction
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
		// If no connection is found for the realm, return the first available connection.
		for _, conn := range p.connectionByRealm {
			return conn
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
	query = strings.TrimSpace(strings.ToUpper(query))

	// Default connection (realm 0)
	conn := p.ConnByRealm(0)
	switch query {
	case StartTransaction:
		p.transactionState = TransactionRequested // Start transaction flag
		return nil, nil
	case CommitTransaction, RollbackTransaction:
		p.transactionState = TransactionNone       // Reset transaction state
		conn = p.ConnByRealm(p.transactionRealmID) // Use realm connection for commit/rollback
	}

	return conn.Execute(query)
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
		GuidFinder: &v,
		Stmts:      map[uint32]DBStatement{},
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

// startTransactionIfNeeded starts a transaction if needed based on the query and state.
func (p *TC9Proxy) startTransactionIfNeeded(args []interface{}, stmtWithParsedData *StmtWithParsedDataContext, realmID uint32) error {
	// Logic to start a transaction if required (for dummy queries or transaction control)
	if stmtWithParsedData.IsDummy && p.transactionState == TransactionRequested {
		if _, err := p.ConnByRealm(0).Execute(StartTransaction); err != nil {
			return fmt.Errorf("failed to start transaction, err: %v", err)
		}

		p.transactionState = TransactionInProgress
		switch v := args[0].(type) {
		case uint64:
			p.transactionRealmID = uint32(v)
		case uint32:
			p.transactionRealmID = v
		case int:
			p.transactionRealmID = uint32(v)
		case uint16:
			p.transactionRealmID = uint32(v)
		case uint8:
			p.transactionRealmID = uint32(v)
		default:
			return fmt.Errorf("invalid realmID context provider argument type: %T", v)
		}
		return nil
	}

	if p.transactionState == TransactionRequested {
		if _, err := p.ConnByRealm(0).Execute(StartTransaction); err != nil {
			return fmt.Errorf("failed to start transaction, err: %v", err)
		}
		p.transactionState = TransactionInProgress
		p.transactionRealmID = realmID
	}

	return nil
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
