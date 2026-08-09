package events

import "fmt"

type PetitionServiceEvent int

const (
	PetitionEventSignResult PetitionServiceEvent = iota + 1
	PetitionEventOffered
	PetitionEventDeclined
)

func (e PetitionServiceEvent) SubjectName() string {
	switch e {
	case PetitionEventSignResult:
		return "petition.sign.result"
	case PetitionEventOffered:
		return "petition.offered"
	case PetitionEventDeclined:
		return "petition.declined"
	}
	panic(fmt.Errorf("unknown petition event %d", e))
}

type PetitionEventSignResultPayload struct {
	ServiceID        string
	RealmID          uint32
	PetitionItemGUID uint64
	OwnerGUID        uint64
	SignerGUID       uint64
	Result           uint32 // AC PetitionSigns
}

type PetitionEventOfferedPayload struct {
	ServiceID        string
	RealmID          uint32
	PetitionItemGUID uint64
	OwnerGUID        uint64
	PetitionID       uint32
	TargetGUID       uint64
	SignatoryGUIDs   []uint64
}

type PetitionEventDeclinedPayload struct {
	ServiceID  string
	RealmID    uint32
	OwnerGUID  uint64
	SignerGUID uint64
}
