package repo

import (
	"fmt"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

// CharsPreparedStatements represents prepared statements for the characters database.
// Implements sharedrepo.PreparedStatement interface.
type CharsPreparedStatements uint32

const (
	// StmtReplaceGroupInvite creates or replaces group invite.
	StmtReplaceGroupInvite CharsPreparedStatements = iota + 1

	// StmtSelectGroupInviteByInvited selects group invite by invited GUID.
	StmtSelectGroupInviteByInvited

	// StmtInsertNewGroup inserts new group record.
	StmtInsertNewGroup

	// StmtUpdateGroupWithID updates group with given ID.
	StmtUpdateGroupWithID

	// StmtInsertNewGroupMember inserts new group member record.
	StmtInsertNewGroupMember

	// StmtUpdateGroupMemberWithID updates group member with given ID.
	StmtUpdateGroupMemberWithID

	// StmtDeleteGroupMemberWithID deletes group member by member ID.
	StmtDeleteGroupMemberWithID

	// StmtDeleteGroupWithID deletes group with given ID.
	StmtDeleteGroupWithID

	// StmtDeleteGroupMembersWithGroupID deletes group members with given guild ID.
	StmtDeleteGroupMembersWithGroupID
)

// ID returns identifier of prepared statement.
func (s CharsPreparedStatements) ID() uint32 {
	return uint32(s)
}

// SchemeStatement returns prepared statement for given schema.
func (s CharsPreparedStatements) SchemeStatement(schemaType shrepo.SupportedSchemaType) shrepo.PreparedStatement {
	switch schemaType {
	case shrepo.SupportedSchemaTypeTrinityCore, shrepo.SupportedSchemaTypeAzerothCore:
		return shrepo.NewGenericPreparedStatement(s.ID(), s.tcAcScheme())
	case shrepo.SupportedSchemaTypeCMaNGOS:
		return shrepo.NewGenericPreparedStatement(s.ID(), s.cmangosScheme())
	}

	panic(fmt.Errorf("unk scheme %s", schemaType))
}
