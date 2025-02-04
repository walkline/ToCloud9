package battleground

import (
	"time"

	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

type QueueTypeID uint8

const (
	QueueTypeIDNone QueueTypeID = iota
	QueueTypeIDAlteracValley
	QueueTypeIDWarsongGulch
	QueueTypeIDArathiBasin
	QueueTypeIDNagrandArena
	QueueTypeIDBladesEdgeArena
	QueueTypeIDAllArenas
	QueueTypeIDEyeOfTheStorm
	QueueTypeIDRuinsOfLordaeron
	QueueTypeIDStrandOfTheAncients
	QueueTypeIDDalaranSewers
	QueueTypeIDRingOfValor
	QueueTypeIDIsleOfConquest     = 30
	QueueTypeIDRandomBattleground = 32

	// QueueTypeIDEnd is end of the list
	QueueTypeIDEnd = 33
)

type TypeID uint8

const (
	TypeIDAlteracValley       = TypeID(QueueTypeIDAlteracValley)
	TypeIDWarsongGulch        = TypeID(QueueTypeIDWarsongGulch)
	TypeIDArathiBasin         = TypeID(QueueTypeIDArathiBasin)
	TypeIDNagrandArena        = TypeID(QueueTypeIDNagrandArena)
	TypeIDBladesEdgeArena     = TypeID(QueueTypeIDBladesEdgeArena)
	TypeIDEyeOfTheStorm       = TypeID(QueueTypeIDEyeOfTheStorm)
	TypeIDRuinsOfLordaeron    = TypeID(QueueTypeIDRuinsOfLordaeron)
	TypeIDStrandOfTheAncients = TypeID(QueueTypeIDStrandOfTheAncients)
	TypeIDDalaranSewers       = TypeID(QueueTypeIDDalaranSewers)
	TypeIDRingOfValor         = TypeID(QueueTypeIDRingOfValor)
	TypeIDIsleOfConquest      = TypeID(QueueTypeIDIsleOfConquest)
)

type InvitedPlayer struct {
	GUID        guid.PlayerUnwrapped
	InvitedTime time.Time
}

type PVPTeam uint8

const (
	TeamAny PVPTeam = iota
	TeamAlliance
	TeamHorde
)

type Status uint8

const (
	StatusNone Status = iota
	StatusWaitQueue
	StatusWaitJoin
	StatusInProgress
	StatusEnded
)

type Battleground struct {
	InstanceID uint32

	MapID uint32

	GameserverAddress string

	BattlegroundTypeID TypeID
	QueueTypeID        QueueTypeID
	BracketID          uint8
	BattleGroupID      uint32
	RealmID            uint32

	MinPlayersPerTeam uint8
	MaxPlayersPerTeam uint8

	Status Status

	ActivePlayersPerTeam  [TeamHorde + 1][]guid.PlayerUnwrapped
	InvitedPlayersPerTeam [TeamHorde + 1][]InvitedPlayer
}

func (b *Battleground) FreeSlotsForTeam(team PVPTeam) uint8 {
	r := int(b.MaxPlayersPerTeam) - (len(b.ActivePlayersPerTeam[team]) + len(b.InvitedPlayersPerTeam[team]))
	if r < 0 {
		return 0
	}

	return uint8(r)
}

func (b *Battleground) InviteGroups(eventsProducer events.MatchmakingServiceProducer, groups []QueuedGroup, team PVPTeam) error {
	groupsByRealm := make(map[uint32][]QueuedGroup)
	for _, group := range groups {
		groupsByRealm[group.RealmID] = append(groupsByRealm[group.RealmID], group)
	}

	now := time.Now()
	for realm, queuedGroups := range groupsByRealm {
		players := make([]guid.LowType, 0, b.MaxPlayersPerTeam)
		slots := map[guid.LowType]uint8{}
		for _, group := range queuedGroups {
			players = append(players, group.LeaderGUID.LowGUID)
			for _, member := range group.Members {
				players = append(players, member.LowGUID)
				slots[member.LowGUID] = group.SlotsPerMember[member]
				b.InvitedPlayersPerTeam[team] = append(b.InvitedPlayersPerTeam[team], InvitedPlayer{
					GUID:        member,
					InvitedTime: now,
				})
			}
			slots[group.LeaderGUID.LowGUID] = group.SlotsPerMember[group.LeaderGUID]
			b.InvitedPlayersPerTeam[team] = append(b.InvitedPlayersPerTeam[team], InvitedPlayer{
				GUID:        group.LeaderGUID,
				InvitedTime: now,
			})
		}

		minLvl, maxLvl := LevelsDiapasonForBracket(b.BracketID)
		v := &events.MatchmakingEventPlayersInvitedPayload{
			RealmID:                  realm,
			PlayersGUID:              players,
			QueueSlotByPlayer:        slots,
			ArenaType:                0,     // TODO: implement later
			IsRated:                  false, // TODO: implement later
			PVPQueueMinLVL:           minLvl,
			PVPQueueMaxLVL:           maxLvl,
			TypeID:                   uint8(b.QueueTypeID),
			MapID:                    b.MapID,
			TimeToAcceptMilliseconds: uint32((time.Minute).Milliseconds()),
		}

		err := eventsProducer.InvitedToBGOrArena(v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Battleground) TeamForInvitedPlayer(playerGUID uint64, realmID uint32) (found bool, team PVPTeam) {
	unwrappedGUID := guid.PlayerUnwrapped{
		RealmID: uint16(realmID),
		LowGUID: guid.LowType(playerGUID),
	}
	for teamIndex, guids := range b.InvitedPlayersPerTeam {
		for _, g := range guids {
			if g.GUID == unwrappedGUID {
				found = true
				team = PVPTeam(teamIndex)
				return
			}
		}
	}
	return false, TeamAny
}

func (b *Battleground) RemovePlayer(playerGUID uint64, realmID uint32) {
	unwrappedGUID := guid.PlayerUnwrapped{
		RealmID: uint16(realmID),
		LowGUID: guid.LowType(playerGUID),
	}

activePlayersLoop:
	for teamIndex, guids := range b.ActivePlayersPerTeam {
		// TODO: add realm check
		for index, guid := range guids {
			if guid == unwrappedGUID {
				b.ActivePlayersPerTeam[teamIndex] = append(guids[:index], guids[index+1:]...)
				break activePlayersLoop
			}
		}
	}

	b.RemovePlayerFromInvite(playerGUID, realmID)
}

func (b *Battleground) RemovePlayerFromInvite(playerGUID uint64, realmID uint32) {
	unwrappedGUID := guid.PlayerUnwrapped{
		RealmID: uint16(realmID),
		LowGUID: guid.LowType(playerGUID),
	}

	for teamIndex, guids := range b.InvitedPlayersPerTeam {
		for index, guid := range guids {
			if guid.GUID == unwrappedGUID {
				b.InvitedPlayersPerTeam[teamIndex] = append(guids[:index], guids[index+1:]...)
				return
			}
		}
	}
}

func (b *Battleground) AddActivePlayer(playerGUID uint64, realmID uint32, team PVPTeam) {
	b.ActivePlayersPerTeam[team] = append(b.ActivePlayersPerTeam[team], guid.PlayerUnwrapped{
		RealmID: uint16(realmID),
		LowGUID: guid.LowType(playerGUID),
	})
}

func (b *Battleground) IsActive() bool {
	return b.Status != StatusEnded
}

func (b *Battleground) DeepCopy() *Battleground {
	// Create a new instance of Battleground
	c := *b

	// Deep c the slices inside the struct
	for i := range b.ActivePlayersPerTeam {
		c.ActivePlayersPerTeam[i] = append([]guid.PlayerUnwrapped(nil), b.ActivePlayersPerTeam[i]...)
	}

	for i := range b.InvitedPlayersPerTeam {
		c.InvitedPlayersPerTeam[i] = append([]InvitedPlayer(nil), b.InvitedPlayersPerTeam[i]...)
	}

	// Return the pointer to the new c
	return &c
}

func BracketIDByLevel(level uint8) uint8 {
	if level < 10 {
		return 1
	}
	if level >= 80 {
		return 8
	}
	return level / 10
}

func LevelsDiapasonForBracket(bracket uint8) (min uint8, max uint8) {
	if bracket >= 8 {
		return 80, 80
	}

	return bracket * 10, bracket*10 + 9
}
