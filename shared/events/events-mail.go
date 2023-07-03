package events

import "fmt"

// MailServiceEvent is event type that mail service generates
type MailServiceEvent int

const (
	// MailEventIncomingMail mail event for a new incoming mail for the character
	MailEventIncomingMail MailServiceEvent = iota + 1
)

// SubjectName is key that nats uses
func (e MailServiceEvent) SubjectName() string {
	switch e {
	case MailEventIncomingMail:
		return "mail.incoming"
	}
	panic(fmt.Errorf("unk event %d", e))
}

// MailEventIncomingMailPayload represents payload of MailEventIncomingMail event
type MailEventIncomingMailPayload struct {
	RealmID uint32

	SenderGUID   uint64
	ReceiverGUID uint64

	DeliveryTimestamp int64

	MailID uint32
}
