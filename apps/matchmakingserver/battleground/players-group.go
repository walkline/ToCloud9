package battleground

import (
	"time"

	"github.com/walkline/ToCloud9/shared/wow/guid"
)

type QueuedGroup struct {
	LeaderGUID guid.PlayerUnwrapped

	// Members includes leader as well
	Members        []guid.PlayerUnwrapped
	SlotsPerMember map[guid.PlayerUnwrapped]uint8

	RealmID uint32
	TeamID  PVPTeam

	ArenaType uint8
	IsRated   bool

	ArenaTeamID                  uint32
	ArenaTeamRating              uint32
	ArenaMatchmakerRating        uint32
	ArenaPreviousOpponentsTeamID uint32

	EnqueuedTime time.Time
}
