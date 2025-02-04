package sqlparser

import (
	"testing"

	"github.com/pingcap/tidb/pkg/parser"
	"github.com/stretchr/testify/assert"
)

func TestCharGUIDFinder_SelectStatement(t *testing.T) {
	sql := `SELECT guid FROM characters WHERE guid = ?`
	p := parser.New()
	stmt, err := p.ParseOneStmt(sql, "", "")
	if err != nil {
		t.Fatalf("Failed to parse SQL: %v", err)
	}

	charGUIDFinder := NewCharGUIDFinder()
	stmt.Accept(&charGUIDFinder)

	charGUIDFinder.FillInGUIDIndexes()

	assert.True(t, charGUIDFinder.IsSelectStmt, "Should identify as a SELECT statement")
	assert.Equal(t, []int{0}, charGUIDFinder.InputGUIDIndexes, "Should find the input GUID index")
	assert.Equal(t, []int{0}, charGUIDFinder.OutputGUIDIndexes, "Should find the output GUID index")
}

func TestCharGUIDFinder_InsertStatement(t *testing.T) {
	sql := `INSERT INTO characters (guid, name) VALUES (?, ?)`
	p := parser.New()
	stmt, err := p.ParseOneStmt(sql, "", "")
	if err != nil {
		t.Fatalf("Failed to parse SQL: %v", err)
	}

	charGUIDFinder := NewCharGUIDFinder()
	stmt.Accept(&charGUIDFinder)

	charGUIDFinder.FillInGUIDIndexes()

	assert.True(t, charGUIDFinder.isInsert, "Should identify as an INSERT statement")
	assert.Equal(t, []int{0}, charGUIDFinder.InputGUIDIndexes, "Should find the input GUID index")
}

func TestCharGUIDFinder_UpdateStatement(t *testing.T) {
	sql := `UPDATE characters SET name = ? WHERE guid = ?`
	p := parser.New()
	stmt, err := p.ParseOneStmt(sql, "", "")
	if err != nil {
		t.Fatalf("Failed to parse SQL: %v", err)
	}

	charGUIDFinder := NewCharGUIDFinder()
	stmt.Accept(&charGUIDFinder)

	charGUIDFinder.FillInGUIDIndexes()

	assert.Equal(t, []int{1}, charGUIDFinder.InputGUIDIndexes, "Should find the input GUID index")
}

func TestCharGUIDFinder_NoGUID(t *testing.T) {
	sql := `SELECT name FROM characters WHERE name = ?`
	p := parser.New()
	stmt, err := p.ParseOneStmt(sql, "", "")
	if err != nil {
		t.Fatalf("Failed to parse SQL: %v", err)
	}

	charGUIDFinder := NewCharGUIDFinder()
	stmt.Accept(&charGUIDFinder)

	charGUIDFinder.FillInGUIDIndexes()

	assert.Empty(t, charGUIDFinder.InputGUIDIndexes, "Should not find any input GUID index")
	assert.Empty(t, charGUIDFinder.OutputGUIDIndexes, "Should not find any output GUID index")
}
