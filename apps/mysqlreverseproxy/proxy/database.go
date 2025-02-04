package proxy

import (
	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
)

// DBConnection represents a generic database connection.
type DBConnection interface {
	Execute(query string, args ...interface{}) (*mysql.Result, error)
	Prepare(query string) (DBStatement, error)
	FieldList(table, fieldWildcard string) ([]*mysql.Field, error)
}

// DBStatement represents a prepared statement.
type DBStatement interface {
	Execute(args ...interface{}) (*mysql.Result, error)
	Close() error
	ParamNum() int
	ColumnNum() int
}

// MySQLConnWrapper wraps a real MySQL client connection.
type MySQLConnWrapper struct {
	conn *client.Conn
}

// NewMySQLConnWrapper creates mysql connection wrapper.
func NewMySQLConnWrapper(conn *client.Conn) DBConnection {
	return &MySQLConnWrapper{conn: conn}
}

// Execute executes a query.
func (c *MySQLConnWrapper) Execute(query string, args ...interface{}) (*mysql.Result, error) {
	return c.conn.Execute(query, args...)
}

// Prepare prepares a statement.
func (c *MySQLConnWrapper) Prepare(query string) (DBStatement, error) {
	stmt, err := c.conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &MySQLStmtWrapper{stmt: stmt}, nil
}

// FieldList retrieves field list metadata.
func (c *MySQLConnWrapper) FieldList(table, fieldWildcard string) ([]*mysql.Field, error) {
	return c.conn.FieldList(table, fieldWildcard)
}

// MySQLStmtWrapper wraps a real MySQL client statement.
type MySQLStmtWrapper struct {
	stmt *client.Stmt
}

// Execute executes a prepared statement.
func (s *MySQLStmtWrapper) Execute(args ...interface{}) (*mysql.Result, error) {
	return s.stmt.Execute(args...)
}

// Close closes the statement.
func (s *MySQLStmtWrapper) Close() error {
	return s.stmt.Close()
}

// ParamNum returns the number of parameters.
func (s *MySQLStmtWrapper) ParamNum() int {
	return s.stmt.ParamNum()
}

// ColumnNum returns the number of columns.
func (s *MySQLStmtWrapper) ColumnNum() int {
	return s.stmt.ColumnNum()
}
