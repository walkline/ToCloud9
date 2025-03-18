package repo

import (
	"fmt"
)

type trinityCoreStatementsBuilder struct{}

func (b trinityCoreStatementsBuilder) StmtForType(t AuthStmtType) string {
	switch t {
	case AuthStmtTypeGetAccountByUsername:
		return "SELECT id, username, salt, verifier, session_key_auth, locked, last_ip FROM account WHERE username = ?"
	}

	panic(fmt.Sprintf("unk stmt type %d", uint8(t)))
}
