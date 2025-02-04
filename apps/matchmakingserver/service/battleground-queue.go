package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/matchmakingserver/battleground"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/repo"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

var (
	ErrPlayerNotFound = errors.New("player not found")
)

type QueuedGroup struct {
	LeaderGUID guid.PlayerUnwrapped

	// Members includes leader as well
	Members        []guid.PlayerUnwrapped
	SlotsPerMember map[guid.PlayerUnwrapped]uint8

	RealmID uint32
	TeamID  battleground.PVPTeam

	EnqueuedTime time.Time

	IsRandomQueue bool
}

type PVPQueue interface {
	AddQueuedGroup(g *QueuedGroup) error
	RemoveQueuedGroup(player guid.PlayerUnwrapped) error
	RemoveAllQueuedGroups()

	GetAllQueuedGroups() []QueuedGroup

	QueuedGroupByPlayer(player guid.PlayerUnwrapped) *QueuedGroup

	GetQueueTypeID() battleground.QueueTypeID
}

type GenericBattlegroundQueue struct {
	BattlegroundTypeID battleground.TypeID
	QueueTypeID        battleground.QueueTypeID
	BracketID          uint8
	BattleGroupID      uint32
	RealmID            uint32

	battleGroundService BattleGroundService
	battleGroundCreator BattlegroundCreator

	mut                        sync.RWMutex
	queuedGroups               map[ /*leaderGUID*/ guid.PlayerUnwrapped]QueuedGroup
	playersGroupLeaderByPlayer map[ /*playerGUID*/ guid.PlayerUnwrapped] /*leaderGUID*/ guid.PlayerUnwrapped
}

func NewGenericBattlegroundQueue(service BattleGroundService, battleGroundCreator BattlegroundCreator, template repo.BattlegroundTemplate, realmID, battlegroupID uint32, bracketID uint8) *GenericBattlegroundQueue {
	return &GenericBattlegroundQueue{
		BattlegroundTypeID:         battleground.TypeID(template.TypeID),
		QueueTypeID:                battleground.QueueTypeID(template.TypeID),
		BracketID:                  bracketID,
		BattleGroupID:              battlegroupID,
		RealmID:                    realmID,
		battleGroundService:        service,
		battleGroundCreator:        battleGroundCreator,
		queuedGroups:               map[guid.PlayerUnwrapped]QueuedGroup{},
		playersGroupLeaderByPlayer: map[guid.PlayerUnwrapped]guid.PlayerUnwrapped{},
	}
}

func (q *GenericBattlegroundQueue) GetQueueTypeID() battleground.QueueTypeID {
	return q.QueueTypeID
}

func (q *GenericBattlegroundQueue) AddQueuedGroup(g *QueuedGroup) error {
	q.mut.Lock()

	groupCopy := *g
	q.queuedGroups[g.LeaderGUID] = groupCopy
	for _, member := range g.Members {
		q.playersGroupLeaderByPlayer[member] = g.LeaderGUID
	}
	q.playersGroupLeaderByPlayer[g.LeaderGUID] = g.LeaderGUID

	q.mut.Unlock()

	return q.process(context.Background())
}

func (q *GenericBattlegroundQueue) RemoveQueuedGroup(player guid.PlayerUnwrapped) error {
	q.mut.Lock()
	defer q.mut.Unlock()

	leaderID, ok := q.playersGroupLeaderByPlayer[player]
	if !ok {
		return ErrPlayerNotFound
	}

	for _, guid := range q.queuedGroups[leaderID].Members {
		delete(q.playersGroupLeaderByPlayer, guid)
	}

	delete(q.queuedGroups, leaderID)

	return nil
}

func (q *GenericBattlegroundQueue) RemoveAllQueuedGroups() {
	q.mut.Lock()
	defer q.mut.Unlock()

	q.queuedGroups = make(map[guid.PlayerUnwrapped]QueuedGroup)
	q.playersGroupLeaderByPlayer = make(map[guid.PlayerUnwrapped]guid.PlayerUnwrapped)
}

func (q *GenericBattlegroundQueue) GetAllQueuedGroups() []QueuedGroup {
	q.mut.RLock()
	defer q.mut.RUnlock()

	res := make([]QueuedGroup, 0, len(q.queuedGroups))
	for _, g := range q.queuedGroups {
		res = append(res, g)
	}
	return res
}

func (q *GenericBattlegroundQueue) QueuedGroupByPlayer(player guid.PlayerUnwrapped) *QueuedGroup {
	q.mut.RLock()
	defer q.mut.RUnlock()

	leaderGuid, ok := q.playersGroupLeaderByPlayer[player]
	if !ok {
		return nil
	}

	group := q.queuedGroups[leaderGuid]
	return &group
}

func (q *GenericBattlegroundQueue) process(ctx context.Context) error {
	battlegroundToFillIn, err := q.battleGroundService.BattlegroundsThatNeedPlayers(
		ctx,
		q.QueueTypeID,
		q.BracketID,
		q.BattleGroupID,
		q.RealmID,
	)
	if err != nil {
		return err
	}

	for _, bg := range battlegroundToFillIn {
		freeSlotsAlliance := bg.FreeSlotsForTeam(battleground.TeamAlliance)
		if freeSlotsAlliance > 0 {
			groupsToInvite := q.findGroupsForGivenSlots(freeSlotsAlliance, battleground.TeamAlliance)
			if len(groupsToInvite) > 0 {
				q.inviteGroupsToBG(groupsToInvite, &bg, battleground.TeamAlliance)
			}
		}

		freeSlotsHorde := bg.FreeSlotsForTeam(battleground.TeamHorde)
		if freeSlotsHorde > 0 {
			groupsToInvite := q.findGroupsForGivenSlots(freeSlotsHorde, battleground.TeamHorde)
			if len(groupsToInvite) > 0 {
				q.inviteGroupsToBG(groupsToInvite, &bg, battleground.TeamHorde)
			}
		}
	}

	// Try to create a new battleground
	template := q.getBattlegroundTemplate()
	allianceGroup, hordeGroup := q.balancedGroups(int(template.MinPlayersPerTeam), int(template.MaxPlayersPerTeam))

	// If not enough groups - do nothing.
	if len(allianceGroup) == 0 || len(hordeGroup) == 0 {
		return nil
	}

	err = q.battleGroundCreator.CreateBattleground(ctx, q.getBattlegroundTemplate(), q.QueueTypeID, BracketID(q.BracketID), q.RealmID, q.BattleGroupID, allianceGroup, hordeGroup)
	if err != nil {
		return fmt.Errorf("failed to create battleground: %w", err)
	}

	for _, g := range hordeGroup {
		q.removeGroupFromQueue(&g)
	}

	for _, g := range allianceGroup {
		q.removeGroupFromQueue(&g)
	}

	return nil
}

func (q *GenericBattlegroundQueue) balancedGroups(minPlayers, maxPlayers int) ([]QueuedGroup, []QueuedGroup) {
	q.mut.RLock()
	defer q.mut.RUnlock()

	var allianceGroups, hordeGroups []QueuedGroup

	// Separate groups by team
	for _, group := range q.queuedGroups {
		switch group.TeamID {
		case battleground.TeamAlliance:
			allianceGroups = append(allianceGroups, group)
		case battleground.TeamHorde:
			hordeGroups = append(hordeGroups, group)
		case battleground.TeamAny:
			// For TeamAny groups, add to both Horde and Alliance temporarily
			allianceGroups = append(allianceGroups, group)
			hordeGroups = append(hordeGroups, group)
		}
	}

	// Sort groups by number of members in descending order
	sort.Slice(allianceGroups, func(i, j int) bool {
		return len(allianceGroups[i].Members) > len(allianceGroups[j].Members)
	})
	sort.Slice(hordeGroups, func(i, j int) bool {
		return len(hordeGroups[i].Members) > len(hordeGroups[j].Members)
	})

	totalMembers := func(groups []QueuedGroup) int {
		total := 0
		for _, group := range groups {
			total += len(group.Members) + 1
		}
		return total
	}

	// Function to find the best combination of groups for a given team
	var findBestGroups func(groups []QueuedGroup, minPlayers, maxPlayers int) []QueuedGroup
	findBestGroups = func(groups []QueuedGroup, minPlayers, maxPlayers int) []QueuedGroup {
		var best []QueuedGroup
		bestTotal := 0
		bestDiff := maxPlayers

		var backtrack func(index int, current []QueuedGroup, currentTotal, currentDiff int)
		backtrack = func(index int, current []QueuedGroup, currentTotal, currentDiff int) {
			if index >= len(groups) {
				// Check if current combination is better than the best found so far
				if currentTotal >= minPlayers && currentTotal <= maxPlayers {
					if currentTotal > bestTotal || (currentTotal == bestTotal && currentDiff < bestDiff) {
						best = append([]QueuedGroup{}, current...)
						bestTotal = currentTotal
						bestDiff = currentDiff
					}
				}
				return
			}

			// Calculate maximum possible players that can be added without exceeding maxPlayers
			maxPossiblePlayers := currentTotal + len(groups[index].Members) + 1

			// Include current group in the combination if it maintains the player balance
			if abs(maxPossiblePlayers-maxPlayers) <= bestDiff {
				backtrack(index+1, append(current, groups[index]),
					maxPossiblePlayers,
					abs(maxPossiblePlayers-maxPlayers))
			}

			// Exclude current group from the combination
			backtrack(index+1, current, currentTotal, currentDiff)
		}

		// Start backtracking from the beginning of the groups slice
		backtrack(0, []QueuedGroup{}, 0, maxPlayers)

		return best
	}

	// Find the best combination of groups for Alliance
	bestAlliance := findBestGroups(allianceGroups, minPlayers, maxPlayers)

	// Remove selected Alliance groups from hordeGroups
	for _, group := range bestAlliance {
		for i, hg := range hordeGroups {
			if hg.LeaderGUID == group.LeaderGUID {
				hordeGroups = append(hordeGroups[:i], hordeGroups[i+1:]...)
				break
			}
		}
	}

	// Find the best combination of groups for Horde with remaining groups
	bestHorde := findBestGroups(hordeGroups, minPlayers, maxPlayers)
	totalBestHorde := totalMembers(bestHorde)
	totalBestAlliance := totalMembers(bestAlliance)

	if abs(totalBestHorde-totalBestAlliance) > 1 {
		if totalBestHorde > totalBestAlliance {
			bestHorde = findBestGroups(hordeGroups, minPlayers, totalBestAlliance+1)
		} else {
			bestAlliance = findBestGroups(allianceGroups, minPlayers, totalBestHorde+1)
		}
	}

	return bestAlliance, bestHorde
}

func (q *GenericBattlegroundQueue) findGroupsForGivenSlots(slots uint8, team battleground.PVPTeam) []QueuedGroup {
	q.mut.RLock()
	defer q.mut.RUnlock()

	groups := make([]QueuedGroup, 0)
	for _, group := range q.queuedGroups {
		if slots == 0 {
			break
		}

		if !(group.TeamID == team || group.TeamID == battleground.TeamAny) || uint8(len(group.Members)) > slots {
			continue
		}

		groups = append(groups, group)
		slots -= uint8(len(group.Members))
	}

	return groups
}

func (q *GenericBattlegroundQueue) inviteGroupsToBG(groups []QueuedGroup, bg *battleground.Battleground, team battleground.PVPTeam) {
	if err := q.battleGroundService.InviteGroups(context.Background(), groups, bg, team); err != nil {
		log.Err(err).Msg("failed to invite to existing bg")
		return
	}

	for _, group := range groups {
		q.removeGroupFromQueue(&group)
	}
}

func (q *GenericBattlegroundQueue) removeGroupFromQueue(group *QueuedGroup) {
	q.mut.Lock()
	defer q.mut.Unlock()

	for _, member := range group.Members {
		delete(q.playersGroupLeaderByPlayer, member)
	}

	delete(q.queuedGroups, group.LeaderGUID)
}

func (q *GenericBattlegroundQueue) getBattlegroundTemplate() repo.BattlegroundTemplate {
	// Use BattlegroundTypeID to make random queue work
	return q.battleGroundService.TemplateForQueueTypeID(context.Background(), battleground.QueueTypeID(q.BattlegroundTypeID))
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
