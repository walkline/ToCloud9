package events

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

//go:generate mockery --name=GuildServiceProducer
type GuildServiceProducer interface {
	InviteCreated(payload *GuildEventInviteCreatedPayload) error

	MemberAdded(payload *GuildEventMemberAddedPayload) error
	MemberLeft(payload *GuildEventMemberLeftPayload) error
	MemberKicked(payload *GuildEventMemberKickedPayload) error

	RankCreated(payload *GuildEventRankCreatedPayload) error
	RankUpdated(payload *GuildEventRankUpdatedPayload) error
	RankDeleted(payload *GuildEventRankDeletedPayload) error

	MemberPromote(payload *GuildEventMemberPromotePayload) error
	MemberDemote(payload *GuildEventMemberDemotePayload) error

	MemberNoteUpdated(payload *GuildEventMembersNoteUpdatedPayload) error
	MemberOfficerNoteUpdated(payload *GuildEventMembersOfficerNoteUpdatedPayload) error

	MOTDUpdated(payload *GuildEventMOTDUpdatedPayload) error
	GuildInfoUpdated(payload *GuildEventGuildInfoUpdatedPayload) error

	NewMessage(payload *GuildEventNewMessagePayload) error
}

type guildServiceProducerNatsJSON struct {
	conn *nats.Conn
	ver  string
}

func NewGuildServiceProducerNatsJSON(conn *nats.Conn, ver string) GuildServiceProducer {
	return &guildServiceProducerNatsJSON{
		conn: conn,
		ver:  ver,
	}
}

func (s *guildServiceProducerNatsJSON) InviteCreated(payload *GuildEventInviteCreatedPayload) error {
	return s.publish(GuildEventInviteCreated, payload)
}

func (s *guildServiceProducerNatsJSON) MemberAdded(payload *GuildEventMemberAddedPayload) error {
	return s.publish(GuildEventMemberAdded, payload)
}

func (s *guildServiceProducerNatsJSON) MemberLeft(payload *GuildEventMemberLeftPayload) error {
	return s.publish(GuildEventMemberLeft, payload)
}

func (s *guildServiceProducerNatsJSON) MemberKicked(payload *GuildEventMemberKickedPayload) error {
	return s.publish(GuildEventMemberKicked, payload)
}

func (s *guildServiceProducerNatsJSON) RankCreated(payload *GuildEventRankCreatedPayload) error {
	return s.publish(GuildEventRankCreated, payload)
}

func (s *guildServiceProducerNatsJSON) RankUpdated(payload *GuildEventRankUpdatedPayload) error {
	return s.publish(GuildEventRankUpdated, payload)
}

func (s *guildServiceProducerNatsJSON) RankDeleted(payload *GuildEventRankDeletedPayload) error {
	return s.publish(GuildEventRankDeleted, payload)
}

func (s *guildServiceProducerNatsJSON) MemberPromote(payload *GuildEventMemberPromotePayload) error {
	return s.publish(GuildEventMemberPromote, payload)
}

func (s *guildServiceProducerNatsJSON) MemberDemote(payload *GuildEventMemberDemotePayload) error {
	return s.publish(GuildEventMemberDemote, payload)
}

func (s *guildServiceProducerNatsJSON) MemberNoteUpdated(payload *GuildEventMembersNoteUpdatedPayload) error {
	return s.publish(GuildEventMemberNoteUpdated, payload)
}

func (s *guildServiceProducerNatsJSON) MemberOfficerNoteUpdated(payload *GuildEventMembersOfficerNoteUpdatedPayload) error {
	return s.publish(GuildEventMemberOfficersNoteUpdated, payload)
}

func (s *guildServiceProducerNatsJSON) MOTDUpdated(payload *GuildEventMOTDUpdatedPayload) error {
	return s.publish(GuildEventMOTDUpdated, payload)
}

func (s *guildServiceProducerNatsJSON) GuildInfoUpdated(payload *GuildEventGuildInfoUpdatedPayload) error {
	return s.publish(GuildEventGuildInfoUpdated, payload)
}

func (s *guildServiceProducerNatsJSON) NewMessage(payload *GuildEventNewMessagePayload) error {
	return s.publish(GuildEventNewMessage, payload)
}

func (s *guildServiceProducerNatsJSON) publish(e GuildServiceEvent, payload interface{}) error {
	msg := EventToSendGenericPayload{
		Version:   s.ver,
		EventType: int(e),
		Payload:   payload,
	}

	d, err := json.Marshal(&msg)
	if err != nil {
		return err
	}

	return s.conn.Publish(e.SubjectName(), d)
}
