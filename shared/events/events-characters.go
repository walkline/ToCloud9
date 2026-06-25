package events

import "fmt"

// CharactersServiceEvent is event type that characters service generates
type CharactersServiceEvent int

const (
	// CharEventCharsDisconnectedUnhealthyGW event that contains players that were connected to unhealthy gateway
	CharEventCharsDisconnectedUnhealthyGW CharactersServiceEvent = iota + 1

	// CharEventArenaTeamInviteCreated is emitted when charserver accepts an arena team invite request.
	CharEventArenaTeamInviteCreated

	// CharEventArenaTeamNativeEvent is emitted after a committed arena team mutation.
	CharEventArenaTeamNativeEvent
)

// SubjectName is key that nats uses
func (e CharactersServiceEvent) SubjectName() string {
	switch e {
	case CharEventCharsDisconnectedUnhealthyGW:
		return "char.chars.unhealthy.gw"
	case CharEventArenaTeamInviteCreated:
		return "char.arena-team.invite-created"
	case CharEventArenaTeamNativeEvent:
		return "char.arena-team.native-event"
	}
	panic(fmt.Errorf("unk event %d", e))
}

// CharEventCharsDisconnectedUnhealthyGWPayload represents payload of CharEventCharsDisconnectedUnhealthyGW event
type CharEventCharsDisconnectedUnhealthyGWPayload struct {
	RealmID           uint32
	GatewayID         string
	EventTimeUnixNano uint64
	CharactersGUID    []uint64
}

type CharEventArenaTeamInviteCreatedPayload struct {
	RealmID     uint32
	TargetGUID  uint64
	TargetName  string
	InviterGUID uint64
	InviterName string
	ArenaTeamID uint32
	TeamName    string
}

type CharEventArenaTeamNativeEventPayload struct {
	RealmID       uint32
	ReceiverGUIDs []uint64
	ArenaTeamID   uint32
	Event         uint8
	EventGUID     uint64
	Args          []string
}
