package events

import "fmt"

// ChatServiceEvent is event type that chat service generates
type ChatServiceEvent int

const (
	// ChatEventIncomingWhisper chat event for a new incoming whisper for the character
	ChatEventIncomingWhisper ChatServiceEvent = iota + 1
)

// SubjectName is key that nats uses
func (e ChatServiceEvent) SubjectName(loadBalancerID string) string {
	switch e {
	case ChatEventIncomingWhisper:
		return fmt.Sprintf("chat.lb.%s.income.whisper", loadBalancerID)
	}
	panic(fmt.Errorf("unk event %d", e))
}

// ChatEventIncomingWhisperPayload represents payload of ChatEventIncomingWhisper event
type ChatEventIncomingWhisperPayload struct {
	SenderGUID   uint64
	SenderName   string
	SenderRace   uint8
	ReceiverGUID uint64
	ReceiverName string
	Language     uint32
	Msg          string
}
