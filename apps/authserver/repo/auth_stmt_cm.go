package repo

import (
	"fmt"
)

type cmangosStatementsBuilder struct{}

func (b cmangosStatementsBuilder) StmtForType(t AuthStmtType) string {
	switch t {
	case AuthStmtTypeGetAccountByUsername:
		return "SELECT id, username, UNHEX(s), UNHEX(v), UNHEX(sessionkey), locked, IFNULL(last_ip, '') FROM account WHERE username = ?"
	case AuthStmtTypeUpdateAccountByID:
		return "UPDATE account SET username = ?, s = HEX(?), v = HEX(?), sessionkey = HEX(?), locked = ?, last_ip = ? WHERE id = ?"
	case AuthStmtTypeGetRealmList:
		return "SELECT id, name, icon, realmflags, timezone, allowedSecurityLevel, CAST(realmbuilds AS SIGNED) FROM realmlist"
	case AuthStmtTypeGetCharactersCountOnRealmsByAccount:
		return "SELECT realmid, numchars FROM realmcharacters WHERE acctid = ?"
	}

	panic(fmt.Sprintf("unk stmt type %d", uint8(t)))
}
