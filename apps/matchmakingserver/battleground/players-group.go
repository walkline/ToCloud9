package battleground

import (
	"time"

	"github.com/walkline/ToCloud9/shared/wow/guid"
)

type QueuedGroup struct {
	LeaderGUID guid.PlayerUnwrapped

	// Members holds the party members, excluding the leader.
	Members        []guid.PlayerUnwrapped
	SlotsPerMember map[guid.PlayerUnwrapped]uint8

	RealmID uint32
	TeamID  PVPTeam

	EnqueuedTime time.Time
}
