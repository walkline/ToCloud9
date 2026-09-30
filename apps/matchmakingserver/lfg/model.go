package lfg

import "time"

type Role uint8

const (
	RoleTank Role = 1 << iota
	RoleHealer
	RoleDamage
)

type PlayerKey struct {
	RealmID uint32
	GUID    uint64
}

type Member struct {
	PlayerKey
	Roles            Role
	Level            uint8
	Class            uint8
	EligibleDungeons map[uint32]struct{}
}

type Entry struct {
	RequestID        string
	BattlegroupID    uint32
	Leader           PlayerKey
	Members          []Member
	SelectedDungeons map[uint32]struct{}
	QueuedAt         time.Time
}

type Assignment struct {
	Player PlayerKey
	Role   Role
}

type Proposal struct {
	ID          uint64
	DungeonID   uint32
	Entries     []*Entry
	Assignments []Assignment
	CreatedAt   time.Time
}

type Status uint8

const (
	StatusNone Status = iota
	StatusQueued
	StatusProposed
)

type PlayerStatus struct {
	Status       Status
	QueuedAt     time.Time
	ProposalID   uint64
	DungeonID    uint32
	AssignedRole Role
}
