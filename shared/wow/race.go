package wow

import (
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
)

type Team uint8

const (
	TeamHorde Team = iota + 1
	TeamAlliance
)

// TeamID returns the team ID enum for protocol usage
func (t Team) TeamID() pbChat.TeamID {
	switch t {
	case TeamAlliance:
		return pbChat.TeamID_TEAM_ALLIANCE
	case TeamHorde:
		return pbChat.TeamID_TEAM_HORDE
	default:
		return pbChat.TeamID_TEAM_NEUTRAL
	}
}

type Race struct {
	ID   RaceID
	Team Team
}

type RaceID uint8

const (
	RaceIDHuman RaceID = iota + 1
	RaceIDOrc
	RaceIDDwarf
	RaceIDNightElf
	RaceIDUndead
	RaceIDTauren
	RaceIDGnome
	RaceIDTroll
	RaceIDGoblin
	RaceIDBloodElf
	RaceIDDreanei
)

var DefaultRaces = []Race{
	RaceIDHuman: {
		ID:   RaceIDHuman,
		Team: TeamAlliance,
	},
	RaceIDOrc: {
		ID:   RaceIDOrc,
		Team: TeamHorde,
	},
	RaceIDDwarf: {
		ID:   RaceIDDwarf,
		Team: TeamAlliance,
	},
	RaceIDNightElf: {
		ID:   RaceIDNightElf,
		Team: TeamAlliance,
	},
	RaceIDUndead: {
		ID:   RaceIDUndead,
		Team: TeamHorde,
	},
	RaceIDTauren: {
		ID:   RaceIDTauren,
		Team: TeamHorde,
	},
	RaceIDGnome: {
		ID:   RaceIDGnome,
		Team: TeamAlliance,
	},
	RaceIDTroll: {
		ID:   RaceIDTroll,
		Team: TeamHorde,
	},
	RaceIDGoblin: {
		ID:   RaceIDGoblin,
		Team: TeamAlliance,
	},
	RaceIDBloodElf: {
		ID:   RaceIDBloodElf,
		Team: TeamHorde,
	},
	RaceIDDreanei: {
		ID:   RaceIDDreanei,
		Team: TeamAlliance,
	},
}
