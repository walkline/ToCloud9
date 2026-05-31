package events

import "fmt"

// ChatServiceEvent is event type that chat service generates
type ChatServiceEvent int

const (
	// ChatEventIncomingWhisper chat event for a new incoming whisper for the character
	ChatEventIncomingWhisper ChatServiceEvent = iota + 1
	// ChatEventChannelMessage chat event for a new channel message
	ChatEventChannelMessage
	// ChatEventChannelJoined event when a player joins a channel
	ChatEventChannelJoined
	// ChatEventChannelLeft event when a player leaves a channel
	ChatEventChannelLeft
	// ChatEventChannelNotification general channel notification (kick, ban, mode change, etc.)
	ChatEventChannelNotification
)

// SubjectName is key that nats uses
func (e ChatServiceEvent) SubjectName(gatewayID string) string {
	switch e {
	case ChatEventIncomingWhisper:
		return fmt.Sprintf("chat.gw.%s.income.whisper", gatewayID)
	case ChatEventChannelMessage:
		return fmt.Sprintf("chat.gw.%s.channel.message", gatewayID)
	case ChatEventChannelJoined:
		return fmt.Sprintf("chat.gw.%s.channel.joined", gatewayID)
	case ChatEventChannelLeft:
		return fmt.Sprintf("chat.gw.%s.channel.left", gatewayID)
	case ChatEventChannelNotification:
		return fmt.Sprintf("chat.gw.%s.channel.notify", gatewayID)
	}
	panic(fmt.Errorf("unk event %d", e))
}

// ChatEventIncomingWhisperPayload represents payload of ChatEventIncomingWhisper event
type ChatEventIncomingWhisperPayload struct {
	SenderRealmID   uint32
	SenderGUID      uint64
	SenderName      string
	SenderRace      uint8
	SenderClass     uint8
	SenderGender    uint8
	SenderChatTag   uint8
	ReceiverRealmID uint32
	ReceiverGUID    uint64
	ReceiverName    string
	Language        uint32
	Msg             string
}

// ChatEventChannelMessagePayload represents payload of ChatEventChannelMessage event
type ChatEventChannelMessagePayload struct {
	RealmID       uint32
	ChannelName   string
	ChannelID     uint32
	TeamID        uint32
	SenderGUID    uint64
	SenderName    string
	Language      uint32
	Message       string
	SenderChatTag uint8
}

// ChatEventChannelJoinedPayload represents payload of ChatEventChannelJoined event
type ChatEventChannelJoinedPayload struct {
	ServiceID    string
	RealmID      uint32
	ChannelName  string
	ChannelID    uint32
	ChannelFlags uint32
	TeamID       uint32
	NumMembers   uint32
	PlayerGUID   uint64
	PlayerName   string
	PlayerFlags  uint8 // MEMBER_FLAG_*
}

// ChatEventChannelLeftPayload represents payload of ChatEventChannelLeft event
type ChatEventChannelLeftPayload struct {
	ServiceID    string
	RealmID      uint32
	ChannelName  string
	ChannelID    uint32
	ChannelFlags uint32
	TeamID       uint32
	NumMembers   uint32
	PlayerGUID   uint64
	PlayerName   string
	Silent       bool
}

// ChatEventChannelNotificationPayload represents payload of ChatEventChannelNotification event
type ChatEventChannelNotificationPayload struct {
	RealmID       uint32
	ChannelName   string
	ChannelID     uint32
	ChannelFlags  uint32
	TeamID        uint32
	NumMembers    uint32
	NotifyType    uint8 // ChatNotify type from Channel.h
	TargetGUID    uint64
	TargetName    string
	SecondGUID    uint64 // For notifications that need 2 players
	OldFlags      uint8
	NewFlags      uint8
	ExtraData     string // For additional text data
	AffectsPlayer uint64 // GUID of player who should receive this notification (0 = all)
}
