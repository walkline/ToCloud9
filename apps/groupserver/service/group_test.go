package service

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/walkline/ToCloud9/apps/groupserver/repo"
)

// Concurrent leaves on the same group raced: both goroutines could take the
// disband path (the second one panicked in the cache) or overwrite each
// other's member removal, leaving a group without members that its players
// were still mapped to - impossible to invite them ever again.
func TestGroupsServiceConcurrentLeaves(t *testing.T) {
	for i := 0; i < 50; i++ {
		cache := newWarmedUpCache(t)
		ctx := context.Background()

		assert.NoError(t, cache.Create(ctx, 1, newTwoMembersGroup()))
		assert.NoError(t, cache.AddMember(ctx, 1, &repo.GroupMember{GroupID: 1, MemberGUID: 3, MemberName: "Third", IsOnline: true}))

		s := NewGroupsService(cache, nil, noopGroupProducer{})

		var wg sync.WaitGroup
		start := make(chan struct{})
		leave := func(player uint64) {
			defer wg.Done()
			<-start
			assert.NoError(t, s.Leave(ctx, 1, player))
		}
		wg.Add(2)
		go leave(2)
		go leave(3)
		close(start)
		wg.Wait()

		// One leave shrinks the group to two members, the other one disbands it.
		group, err := s.GroupByID(ctx, 1, 1)
		assert.NoError(t, err)
		assert.Nil(t, group, "group should be disbanded")

		for _, player := range []uint64{1, 2, 3} {
			groupID, err := s.GroupIDByPlayer(ctx, 1, player)
			assert.NoError(t, err)
			assert.Zero(t, groupID, "player %d should not be mapped to a group anymore", player)
		}
	}
}

// staleInviteRepo hands out an invite pointing to a group that no longer exists.
type staleInviteRepo struct{ noopGroupsRepo }

func (staleInviteRepo) GetInviteByInvitedPlayer(ctx context.Context, realmID uint32, invitedPlayer uint64) (*repo.GroupInvite, error) {
	return &repo.GroupInvite{Inviter: 1, InviterName: "Leader", Invitee: invitedPlayer, InviteeName: "Late", GroupID: 999}, nil
}

// Invites are never deleted, only replaced, so accepting one that outlived its
// group used to dereference a nil group and crash the service.
func TestGroupsServiceAcceptStaleInvite(t *testing.T) {
	cache := NewInMemGroupsCache(staleInviteRepo{})
	assert.NoError(t, cache.Warmup(context.Background(), 1))

	s := NewGroupsService(cache, nil, noopGroupProducer{})

	assert.ErrorIs(t, s.AcceptInvite(context.Background(), 1, 2), ErrGroupNotFound)
}
