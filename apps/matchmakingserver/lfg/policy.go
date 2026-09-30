package lfg

import (
	"errors"
	"sort"
)

var ErrNoMatch = errors.New("no lfg match")

// MatchPolicy is the intentionally small extension point for optional future
// composition policies. The default remains Blizzard-like: FIFO candidates,
// one tank, one healer, three damage and a common eligible dungeon.
type MatchPolicy interface {
	Match(entries []*Entry) (*Proposal, error)
}

type BlizzlikePolicy struct{}

func (BlizzlikePolicy) Match(entries []*Entry) (*Proposal, error) {
	members := make([]Member, 0, 5)
	for _, entry := range entries {
		members = append(members, entry.Members...)
	}
	if len(members) != 5 {
		return nil, ErrNoMatch
	}

	dungeons := commonDungeons(entries, members)
	if len(dungeons) == 0 {
		return nil, ErrNoMatch
	}

	assignments, ok := assignRoles(members)
	if !ok {
		return nil, ErrNoMatch
	}

	// Selection is deterministic, which keeps tests and replay behavior stable.
	sort.Slice(dungeons, func(i, j int) bool { return dungeons[i] < dungeons[j] })
	return &Proposal{DungeonID: dungeons[0], Entries: entries, Assignments: assignments}, nil
}

func commonDungeons(entries []*Entry, members []Member) []uint32 {
	common := make(map[uint32]struct{})
	for dungeon := range entries[0].SelectedDungeons {
		common[dungeon] = struct{}{}
	}
	for _, entry := range entries[1:] {
		intersect(common, entry.SelectedDungeons)
	}
	for _, member := range members {
		intersect(common, member.EligibleDungeons)
	}

	result := make([]uint32, 0, len(common))
	for dungeon := range common {
		result = append(result, dungeon)
	}
	return result
}

func intersect(left map[uint32]struct{}, right map[uint32]struct{}) {
	for value := range left {
		if _, found := right[value]; !found {
			delete(left, value)
		}
	}
}

func assignRoles(members []Member) ([]Assignment, bool) {
	assignments := make([]Assignment, len(members))
	var search func(index, tanks, healers, damage int) bool
	search = func(index, tanks, healers, damage int) bool {
		if tanks > 1 || healers > 1 || damage > 3 {
			return false
		}
		if index == len(members) {
			return tanks == 1 && healers == 1 && damage == 3
		}

		member := members[index]
		for _, role := range []Role{RoleTank, RoleHealer, RoleDamage} {
			if member.Roles&role == 0 {
				continue
			}
			assignments[index] = Assignment{Player: member.PlayerKey, Role: role}
			nextTanks, nextHealers, nextDamage := tanks, healers, damage
			switch role {
			case RoleTank:
				nextTanks++
			case RoleHealer:
				nextHealers++
			case RoleDamage:
				nextDamage++
			}
			if search(index+1, nextTanks, nextHealers, nextDamage) {
				return true
			}
		}
		return false
	}

	return assignments, search(0, 0, 0, 0)
}
