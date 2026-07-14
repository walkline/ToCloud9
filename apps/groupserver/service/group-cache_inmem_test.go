package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/walkline/ToCloud9/apps/groupserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

func buildLoggedOutPayload(realmID uint32, charGUID uint64) events.GWEventCharacterLoggedOutPayload {
	return events.GWEventCharacterLoggedOutPayload{
		RealmID:  realmID,
		CharGUID: charGUID,
	}
}

// noopGroupsRepo is a stub persistent repo, cache behaviour is tested in memory only.
type noopGroupsRepo struct{}

func (noopGroupsRepo) LoadAllForRealm(ctx context.Context, realmID uint32) (map[uint]*repo.Group, error) {
	return map[uint]*repo.Group{}, nil
}
func (noopGroupsRepo) GroupByID(ctx context.Context, realmID uint32, partyID uint, loadMembers bool) (*repo.Group, error) {
	return nil, nil
}
func (noopGroupsRepo) GroupIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint, error) {
	return 0, nil
}
func (noopGroupsRepo) Create(ctx context.Context, realmID uint32, group *repo.Group) error { return nil }
func (noopGroupsRepo) Delete(ctx context.Context, realmID uint32, groupID uint) error      { return nil }
func (noopGroupsRepo) Update(ctx context.Context, realmID uint32, group *repo.Group) error { return nil }
func (noopGroupsRepo) AddMember(ctx context.Context, realmID uint32, groupMember *repo.GroupMember) error {
	return nil
}
func (noopGroupsRepo) UpdateMember(ctx context.Context, realmID uint32, groupMember *repo.GroupMember) error {
	return nil
}
func (noopGroupsRepo) RemoveMember(ctx context.Context, realmID uint32, memberGUID uint64) error {
	return nil
}
func (noopGroupsRepo) AddInvite(ctx context.Context, realmID uint32, invite repo.GroupInvite) error {
	return nil
}
func (noopGroupsRepo) GetInviteByInvitedPlayer(ctx context.Context, realmID uint32, invitedPlayer uint64) (*repo.GroupInvite, error) {
	return nil, nil
}

func newWarmedUpCache(t *testing.T) GroupsCache {
	cache := NewInMemGroupsCache(noopGroupsRepo{})
	assert.NoError(t, cache.Warmup(context.Background(), 1))
	return cache
}

func newTwoMembersGroup() *repo.Group {
	return &repo.Group{
		ID:         1,
		LeaderGUID: 1,
		Members: []repo.GroupMember{
			{GroupID: 1, MemberGUID: 1, MemberName: "Leader", IsOnline: true},
			{GroupID: 1, MemberGUID: 2, MemberName: "Second", IsOnline: true},
		},
	}
}

// Adding a member grows the members slice, which can reallocate its backing
// array. Cached member pointers must follow, otherwise online status updates
// are applied to stale copies and never observed through GroupByID.
func TestGroupsCacheInMemMemberStatusUpdatesAfterAddMember(t *testing.T) {
	cache := newWarmedUpCache(t)
	ctx := context.Background()

	assert.NoError(t, cache.Create(ctx, 1, newTwoMembersGroup()))
	assert.NoError(t, cache.AddMember(ctx, 1, &repo.GroupMember{GroupID: 1, MemberGUID: 3, MemberName: "Third", IsOnline: true}))

	assert.NoError(t, cache.HandleCharacterLoggedOut(buildLoggedOutPayload(1, 1)))

	group, err := cache.GroupByID(ctx, 1, 1, true)
	assert.NoError(t, err)
	assert.False(t, group.MemberByGUID(1).IsOnline, "logged out member should be offline")
	assert.True(t, group.MemberByGUID(2).IsOnline)
	assert.True(t, group.MemberByGUID(3).IsOnline)
}

// Removing a member shifts the members that follow to new slots. Cached
// member pointers must follow, otherwise status updates for one member are
// applied to another one.
func TestGroupsCacheInMemMemberStatusUpdatesAfterRemoveMember(t *testing.T) {
	cache := newWarmedUpCache(t)
	ctx := context.Background()

	assert.NoError(t, cache.Create(ctx, 1, newTwoMembersGroup()))
	assert.NoError(t, cache.AddMember(ctx, 1, &repo.GroupMember{GroupID: 1, MemberGUID: 3, MemberName: "Third", IsOnline: true}))
	assert.NoError(t, cache.RemoveMember(ctx, 1, 1))

	assert.NoError(t, cache.HandleCharacterLoggedOut(buildLoggedOutPayload(1, 2)))

	group, err := cache.GroupByID(ctx, 1, 1, true)
	assert.NoError(t, err)
	assert.False(t, group.MemberByGUID(2).IsOnline, "logged out member should be offline")
	assert.True(t, group.MemberByGUID(3).IsOnline, "remaining member should stay online")
}

// Two concurrent leaves on the same group can both reach the disband path;
// the second Delete then found no group in the cache and panicked on a nil
// dereference. Delete must be idempotent.
func TestGroupsCacheInMemDeleteIsIdempotent(t *testing.T) {
	cache := newWarmedUpCache(t)
	ctx := context.Background()

	assert.NoError(t, cache.Create(ctx, 1, newTwoMembersGroup()))
	assert.NoError(t, cache.Delete(ctx, 1, 1))
	assert.NoError(t, cache.Delete(ctx, 1, 1))
}
