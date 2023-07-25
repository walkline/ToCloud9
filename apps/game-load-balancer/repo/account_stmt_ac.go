package repo

import (
	"fmt"
)

type azerothCoreStatementsBuilder struct{}

func (b azerothCoreStatementsBuilder) StmtForType(t AuthStmtType) string {
	switch t {
	case AuthStmtTypeGetAccountByUsername:
		return "SELECT id, username, salt, verifier, session_key, locked, last_ip FROM account WHERE username = ?"
	}

	panic(fmt.Sprintf("unk stmt type %d", uint8(t)))
}
