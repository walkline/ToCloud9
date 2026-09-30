package lfg

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestServiceMatchesFIFOWithRolesAndEligibility(t *testing.T) {
	service := NewService("matcher-1", nil)
	base := time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return base.Add(time.Hour) }

	entries := []*Entry{
		entry("old-damage", base, member(1, RoleDamage, 100, 200)),
		entry("tank", base.Add(time.Minute), member(2, RoleTank, 100, 200)),
		entry("healer", base.Add(2*time.Minute), member(3, RoleHealer, 100, 200)),
		entry("damage-2", base.Add(3*time.Minute), member(4, RoleDamage, 100, 200)),
		entry("damage-3", base.Add(4*time.Minute), member(5, RoleDamage, 100, 200)),
	}

	for i, candidate := range entries {
		proposal, err := service.Join(candidate)
		if err != nil {
			t.Fatalf("join %d: %v", i, err)
		}
		if i < len(entries)-1 && proposal != nil {
			t.Fatalf("unexpected proposal after %d entries", i+1)
		}
		if i == len(entries)-1 {
			if proposal == nil {
				t.Fatal("expected proposal")
			}
			if proposal.DungeonID != 100 {
				t.Fatalf("dungeon = %d, want deterministic dungeon 100", proposal.DungeonID)
			}
		}
	}

	status := service.Status(PlayerKey{RealmID: 1, GUID: 1})
	if status.Status != StatusProposed || status.QueuedAt != base || status.AssignedRole != RoleDamage {
		t.Fatalf("unexpected oldest player status: %+v", status)
	}
}

func TestServiceDoesNotMatchPlayersWithoutCommonAttunedDungeon(t *testing.T) {
	service := NewService("matcher-1", nil)
	roles := []Role{RoleTank, RoleHealer, RoleDamage, RoleDamage, RoleDamage}
	for i, role := range roles {
		dungeons := []uint32{100}
		if i == len(roles)-1 {
			dungeons = []uint32{200}
		}
		proposal, err := service.Join(entry(fmt.Sprintf("entry-%d", i), time.Now(), member(uint64(i+1), role, dungeons...)))
		if err != nil {
			t.Fatal(err)
		}
		if proposal != nil {
			t.Fatal("players with no common eligible dungeon must not match")
		}
	}
}

func TestServiceAcceptsFlexibleRoles(t *testing.T) {
	policy := BlizzlikePolicy{}
	proposal, err := policy.Match([]*Entry{
		entry("premade", time.Now(),
			member(1, RoleTank|RoleDamage, 100),
			member(2, RoleHealer|RoleDamage, 100),
			member(3, RoleDamage, 100),
			member(4, RoleDamage, 100),
			member(5, RoleDamage, 100)),
	})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[Role]int{}
	for _, assignment := range proposal.Assignments {
		counts[assignment.Role]++
	}
	if counts[RoleTank] != 1 || counts[RoleHealer] != 1 || counts[RoleDamage] != 3 {
		t.Fatalf("unexpected assignments: %#v", counts)
	}
}

func TestServiceRejectsPlayerInTwoEntries(t *testing.T) {
	service := NewService("matcher-1", nil)
	if _, err := service.Join(entry("first", time.Now(), member(1, RoleDamage, 100))); err != nil {
		t.Fatal(err)
	}
	_, err := service.Join(entry("second", time.Now(), member(1, RoleDamage, 100)))
	if !errors.Is(err, ErrPlayerAlreadyQueued) {
		t.Fatalf("error = %v, want ErrPlayerAlreadyQueued", err)
	}
}

func TestServicePreservesReplayedQueueTime(t *testing.T) {
	service := NewService("matcher-after-restart", nil)
	original := time.Now().Add(-45 * time.Minute).Truncate(time.Millisecond)
	if _, err := service.Join(entry("replayed", original, member(1, RoleDamage, 100))); err != nil {
		t.Fatal(err)
	}
	status := service.Status(PlayerKey{RealmID: 1, GUID: 1})
	if !status.QueuedAt.Equal(original) {
		t.Fatalf("queued at = %v, want %v", status.QueuedAt, original)
	}
}

func TestServiceReturnsProposalForRetriedRequest(t *testing.T) {
	service := NewService("matcher-1", nil)
	roles := []Role{RoleTank, RoleHealer, RoleDamage, RoleDamage, RoleDamage}
	var first *Entry
	var proposal *Proposal
	for i, role := range roles {
		candidate := entry(fmt.Sprintf("request-%d", i), time.Now(), member(uint64(i+1), role, 100))
		if i == 0 {
			first = candidate
		}
		var err error
		proposal, err = service.Join(candidate)
		if err != nil {
			t.Fatal(err)
		}
	}
	if proposal == nil {
		t.Fatal("expected proposal")
	}

	retried, err := service.Join(first)
	if err != nil {
		t.Fatal(err)
	}
	if retried == nil || retried.ID != proposal.ID {
		t.Fatalf("retried proposal = %+v, want ID %d", retried, proposal.ID)
	}
}

func entry(requestID string, queuedAt time.Time, members ...Member) *Entry {
	return &Entry{
		RequestID:        requestID,
		BattlegroupID:    1,
		Leader:           members[0].PlayerKey,
		Members:          members,
		SelectedDungeons: dungeonSet(100, 200),
		QueuedAt:         queuedAt,
	}
}

func member(guid uint64, roles Role, dungeons ...uint32) Member {
	return Member{
		PlayerKey:        PlayerKey{RealmID: 1, GUID: guid},
		Roles:            roles,
		Level:            80,
		Class:            1,
		EligibleDungeons: dungeonSet(dungeons...),
	}
}

func dungeonSet(dungeons ...uint32) map[uint32]struct{} {
	result := make(map[uint32]struct{}, len(dungeons))
	for _, dungeon := range dungeons {
		result[dungeon] = struct{}{}
	}
	return result
}
