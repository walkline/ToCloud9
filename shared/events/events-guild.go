package events

import "fmt"

// GuildServiceEvent is event type that guilds service generates
type GuildServiceEvent int

const (
	// GuildEventInviteCreated guild event when player invite created
	GuildEventInviteCreated GuildServiceEvent = iota + 1

	// GuildEventMemberAdded guild event when new member added to the guild
	GuildEventMemberAdded

	// GuildEventMemberLeft guild event when member left the guild
	GuildEventMemberLeft

	// GuildEventMemberKicked guild event when member kicked from the guild
	GuildEventMemberKicked

	// GuildEventMemberNoteUpdated guild event when public note of the member updated
	GuildEventMemberNoteUpdated

	// GuildEventMemberOfficersNoteUpdated guild event when officers note of the member updated
	GuildEventMemberOfficersNoteUpdated

	// GuildEventRankCreated guild event when new guild rank created
	GuildEventRankCreated

	// GuildEventRankUpdated guild event when guild rank updated
	GuildEventRankUpdated

	// GuildEventRankDeleted guild event when guild rank deleted
	GuildEventRankDeleted

	// GuildEventMemberPromote guild event when guild member promoted
	GuildEventMemberPromote

	// GuildEventMemberDemote guild event when guild member demoted
	GuildEventMemberDemote

	// GuildEventMOTDUpdated guild event when message of the day updated
	GuildEventMOTDUpdated

	// GuildEventGuildInfoUpdated guild event when guild info message updated
	GuildEventGuildInfoUpdated

	// GuildEventNewMessage guild event when guild member sent some message
	GuildEventNewMessage
)

// SubjectName is key that nats uses
func (e GuildServiceEvent) SubjectName() string {
	switch e {
	case GuildEventInviteCreated:
		return "guild.invite.created"
	case GuildEventMemberAdded:
		return "guild.member.added"
	case GuildEventMemberLeft:
		return "guild.member.left"
	case GuildEventMemberKicked:
		return "guild.member.kicked"
	case GuildEventMemberNoteUpdated:
		return "guild.member.noteupdated"
	case GuildEventMemberOfficersNoteUpdated:
		return "guild.member.officernoteupdated"
	case GuildEventRankCreated:
		return "guild.rank.created"
	case GuildEventRankUpdated:
		return "guild.rank.updated"
	case GuildEventRankDeleted:
		return "guild.rank.deleted"
	case GuildEventMemberPromote:
		return "guild.member.promoted"
	case GuildEventMemberDemote:
		return "guild.member.demoted"
	case GuildEventMOTDUpdated:
		return "guild.motd.updated"
	case GuildEventGuildInfoUpdated:
		return "guild.info.updated"
	case GuildEventNewMessage:
		return "guild.message.new"
	}
	panic(fmt.Errorf("unk event %d", e))
}

type GuildEventInviteCreatedPayload struct {
	// ServiceID is identifier of guild service
	ServiceID string
	RealmID   uint32

	GuildID   uint64
	GuildName string

	InviterGUID uint64
	InviterName string

	InviteeGUID uint64
	InviteeName string
}

type GenericGuildEvent struct {
	// ServiceID is identifier of guild service
	ServiceID string
	RealmID   uint32

	GuildID   uint64
	GuildName string

	MembersOnline []uint64
}

type GuildEventMemberAddedPayload struct {
	GenericGuildEvent

	MemberGUID uint64
	MemberName string
}

type GuildEventMemberLeftPayload struct {
	GenericGuildEvent

	MemberGUID uint64
	MemberName string
}

type GuildEventMemberKickedPayload struct {
	GenericGuildEvent

	MemberGUID uint64
	MemberName string

	KickerGUID uint64
	KickerName string
}

type GuildEventMembersNoteUpdatedPayload struct {
	GenericGuildEvent

	MemberGUID uint64
	MemberName string

	UpdaterGUID uint64
	UpdaterName string

	Note string
}

type GuildEventMembersOfficerNoteUpdatedPayload struct {
	GenericGuildEvent

	MemberGUID uint64
	MemberName string

	UpdaterGUID uint64
	UpdaterName string

	Note string
}

type GuildEventRankCreatedPayload struct {
	GenericGuildEvent

	RankID   uint8
	RankName string

	RanksCount uint8
}

type GuildEventRankUpdatedPayload struct {
	GenericGuildEvent

	RankID          uint8
	RankName        string
	RankRights      uint32
	RankMoneyPerDay uint32

	RanksCount uint8
}

type GuildEventRankDeletedPayload struct {
	GenericGuildEvent

	RankID   uint8
	RankName string

	RanksCount uint8
}

type GuildEventMemberPromotePayload struct {
	GenericGuildEvent

	RankID   uint8
	RankName string

	PromoterGUID uint64
	PromoterName string

	MemberGUID uint64
	MemberName string
}

type GuildEventMemberDemotePayload struct {
	GenericGuildEvent

	RankID   uint8
	RankName string

	DemoterGUID uint64
	DemoterName string

	MemberGUID uint64
	MemberName string
}

type GuildEventMOTDUpdatedPayload struct {
	GenericGuildEvent

	NewMessageOfTheDay string
}

type GuildEventGuildInfoUpdatedPayload struct {
	GenericGuildEvent

	NewGuildInfo string
}

type GuildEventNewMessagePayload struct {
	ServiceID string
	RealmID   uint32

	GuildID uint64

	SenderGUID uint64
	SenderName string

	Language uint32
	Msg      string

	ForOfficers bool

	Receivers []uint64
}
