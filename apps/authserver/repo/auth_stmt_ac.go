package repo

import (
	"fmt"
)

type azerothCoreStatementsBuilder struct{}

func (b azerothCoreStatementsBuilder) StmtForType(t AuthStmtType) string {
	switch t {
	case AuthStmtTypeGetAccountByUsername:
		return "SELECT id, username, salt, verifier, session_key, locked, last_ip FROM account WHERE username = ?"
	case AuthStmtTypeUpdateAccountByID:
		return "UPDATE account SET username = ?, salt = ?, verifier = ?, session_key = ?, locked = ?, last_ip = ? WHERE id = ?"
	case AuthStmtTypeGetRealmList:
		return "SELECT id, name, icon, flag, timezone, allowedSecurityLevel, gamebuild FROM realmlist"
	case AuthStmtTypeGetCharactersCountOnRealmsByAccount:
		return "SELECT realmid, numchars FROM realmcharacters WHERE acctid = ?"
	}

	panic(fmt.Sprintf("unk stmt type %d", uint8(t)))
}
