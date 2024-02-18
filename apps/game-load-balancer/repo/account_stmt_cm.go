package repo

import (
	"fmt"
)

type cmangosStatementsBuilder struct{}

func (b cmangosStatementsBuilder) StmtForType(t AuthStmtType) string {
	switch t {
	case AuthStmtTypeGetAccountByUsername:
		return "SELECT id, username, UNHEX(s), UNHEX(v), UNHEX(sessionkey), locked, IFNULL(last_ip, '') FROM account WHERE username = ?"
	}

	panic(fmt.Sprintf("unk stmt type %d", uint8(t)))
}
