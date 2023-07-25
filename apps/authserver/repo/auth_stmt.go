package repo

import "github.com/walkline/ToCloud9/shared/repo"

type AuthStmtType uint8

const (
	AuthStmtTypeGetAccountByUsername AuthStmtType = iota
	AuthStmtTypeUpdateAccountByID

	AuthStmtTypeGetRealmList
	AuthStmtTypeGetCharactersCountOnRealmsByAccount
)

type StatementsBuilder interface {
	StmtForType(AuthStmtType) string
}

func StatementsBuilderForSchema(schemaType repo.SupportedSchemaType) StatementsBuilder {
	switch schemaType {
	case repo.SupportedSchemaTypeTrinityCore:
		return trinityCoreStatementsBuilder{}
	case repo.SupportedSchemaTypeAzerothCore:
		return azerothCoreStatementsBuilder{}
	}
	panic("unk schema type")
}
