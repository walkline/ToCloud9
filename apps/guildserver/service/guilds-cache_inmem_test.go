package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/apps/guildserver/repo/mocks"
	"github.com/walkline/ToCloud9/shared/events"
)

func Test_guildsInMemCache_DeleteLowestGuildRank(t *testing.T) {
	for _, tc := range []struct {
		name          string
		cache         map[uint32]map[uint64]*repo.Guild
		realmID       uint32
		guildID       uint64
		rank          uint8
		expectedCache map[uint32]map[uint64]*repo.Guild
	}{
		{
			name: "remove from the middle",
			cache: map[uint32]map[uint64]*repo.Guild{
				1: {
					1: &repo.Guild{
						GuildRanks: []repo.GuildRank{
							{Rank: 0},
							{Rank: 1},
							{Rank: 2},
							{Rank: 3},
						},
					},
				},
			},
			realmID: 1,
			guildID: 1,
			rank:    2,
			expectedCache: map[uint32]map[uint64]*repo.Guild{
				1: {
					1: &repo.Guild{
						GuildRanks: []repo.GuildRank{
							{Rank: 0},
							{Rank: 1},
						},
					},
				},
			},
		},
		{
			name: "remove last",
			cache: map[uint32]map[uint64]*repo.Guild{
				1: {
					1: &repo.Guild{
						GuildRanks: []repo.GuildRank{
							{Rank: 0},
							{Rank: 1},
							{Rank: 2},
							{Rank: 3},
						},
					},
				},
			},
			realmID: 1,
			guildID: 1,
			rank:    3,
			expectedCache: map[uint32]map[uint64]*repo.Guild{
				1: {
					1: &repo.Guild{
						GuildRanks: []repo.GuildRank{
							{Rank: 0},
							{Rank: 1},
							{Rank: 2},
						},
					},
				},
			},
		},
		{
			name: "rank to high",
			cache: map[uint32]map[uint64]*repo.Guild{
				1: {
					1: &repo.Guild{
						GuildRanks: []repo.GuildRank{
							{Rank: 0},
							{Rank: 1},
							{Rank: 2},
							{Rank: 3},
						},
					},
				},
			},
			realmID: 1,
			guildID: 1,
			rank:    4,
			expectedCache: map[uint32]map[uint64]*repo.Guild{
				1: {
					1: &repo.Guild{
						GuildRanks: []repo.GuildRank{
							{Rank: 0},
							{Rank: 1},
							{Rank: 2},
							{Rank: 3},
						},
					},
				},
			},
		},
		{
			name: "guild not exist",
			cache: map[uint32]map[uint64]*repo.Guild{
				1: {
					1: &repo.Guild{
						GuildRanks: []repo.GuildRank{
							{Rank: 0},
							{Rank: 1},
							{Rank: 2},
							{Rank: 3},
						},
					},
				},
			},
			realmID: 1,
			guildID: 2,
			rank:    3,
			expectedCache: map[uint32]map[uint64]*repo.Guild{
				1: {
					1: &repo.Guild{
						GuildRanks: []repo.GuildRank{
							{Rank: 0},
							{Rank: 1},
							{Rank: 2},
							{Rank: 3},
						},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoMock := &mocks.GuildsRepo{}
			repoMock.On(
				"DeleteLowestGuildRank",
				mock.Anything, mock.Anything,
				mock.Anything, mock.Anything,
			).Return(nil)
			cache := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
			cache.cache = tc.cache
			err := cache.DeleteLowestGuildRank(context.Background(), tc.realmID, tc.guildID, tc.rank)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedCache, cache.cache)
		})
	}

}

func Test_guildsInMemCache_GuildIDByRealmAndMemberGUIDFromSource(t *testing.T) {
	const (
		realmID    = uint32(1)
		guildID    = uint64(65)
		memberGUID = uint64(42)
	)

	newCacheWithMember := func(repoMock *mocks.GuildsRepo) *guildsInMemCache {
		cache := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
		member := &repo.GuildMember{PlayerGUID: memberGUID, GuildID: guildID}
		cache.cache = map[uint32]map[uint64]*repo.Guild{
			realmID: {guildID: &repo.Guild{ID: guildID, GuildMembers: []*repo.GuildMember{member}}},
		}
		cache.guildMembersCache = map[uint32]map[uint64]*repo.GuildMember{
			realmID: {memberGUID: member},
		}
		return cache
	}

	t.Run("evicts stale member when source has no membership", func(t *testing.T) {
		repoMock := &mocks.GuildsRepo{}
		repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, realmID, memberGUID).Return(uint64(0), nil)

		cache := newCacheWithMember(repoMock)
		id, err := cache.GuildIDByRealmAndMemberGUIDFromSource(context.Background(), realmID, memberGUID)
		assert.NoError(t, err)
		assert.Equal(t, uint64(0), id)
		assert.Nil(t, cache.guildMembersCache[realmID][memberGUID])
		assert.Empty(t, cache.cache[realmID][guildID].GuildMembers)
	})

	t.Run("keeps member when source confirms membership", func(t *testing.T) {
		repoMock := &mocks.GuildsRepo{}
		repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, realmID, memberGUID).Return(guildID, nil)

		cache := newCacheWithMember(repoMock)
		id, err := cache.GuildIDByRealmAndMemberGUIDFromSource(context.Background(), realmID, memberGUID)
		assert.NoError(t, err)
		assert.Equal(t, guildID, id)
		assert.NotNil(t, cache.guildMembersCache[realmID][memberGUID])
		assert.Len(t, cache.cache[realmID][guildID].GuildMembers, 1)
	})
}

func Test_guildsInMemCache_GuildByRealmAndIDRefreshesFromSource(t *testing.T) {
	const (
		realmID    = uint32(1)
		guildID    = uint64(64)
		leaderGUID = uint64(50553)
		cutsGUID   = uint64(50554)
		offlineMemberGUID = uint64(50555)
	)

	newSeededCache := func(repoMock *mocks.GuildsRepo) *guildsInMemCache {
		cache := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
		leader := &repo.GuildMember{PlayerGUID: leaderGUID, GuildID: guildID, Status: repo.GuildMemberStatusOnline}
		cache.cache = map[uint32]map[uint64]*repo.Guild{
			realmID: {guildID: &repo.Guild{ID: guildID, GuildMembers: []*repo.GuildMember{leader}}},
		}
		cache.guildMembersCache = map[uint32]map[uint64]*repo.GuildMember{
			realmID: {leaderGUID: leader},
		}
		return cache
	}

	t.Run("picks up members added in-process and overlays online status", func(t *testing.T) {
		repoMock := &mocks.GuildsRepo{}
		repoMock.On("GuildByRealmAndID", mock.Anything, realmID, guildID).Return(&repo.Guild{
			ID: guildID,
			GuildMembers: []*repo.GuildMember{
				{PlayerGUID: leaderGUID, GuildID: guildID, Status: repo.GuildMemberStatusOffline, LogoutTime: 10},
				{PlayerGUID: cutsGUID, GuildID: guildID, Status: repo.GuildMemberStatusOffline, LogoutTime: 20},
				{PlayerGUID: offlineMemberGUID, GuildID: guildID, Status: repo.GuildMemberStatusOffline, LogoutTime: 30},
			},
		}, nil)

		cache := newSeededCache(repoMock)
		assert.NoError(t, cache.HandleCharacterLoggedIn(events.GWEventCharacterLoggedInPayload{RealmID: realmID, CharGUID: leaderGUID}))
		assert.NoError(t, cache.HandleCharacterLoggedIn(events.GWEventCharacterLoggedInPayload{RealmID: realmID, CharGUID: offlineMemberGUID}))

		guild, err := cache.GuildByRealmAndID(context.Background(), realmID, guildID)
		assert.NoError(t, err)
		assert.Len(t, guild.GuildMembers, 3)

		statuses := map[uint64]repo.GuildMemberStatus{}
		for _, member := range guild.GuildMembers {
			statuses[member.PlayerGUID] = member.Status
		}
		assert.Equal(t, repo.GuildMemberStatusOnline, statuses[leaderGUID])
		assert.Equal(t, repo.GuildMemberStatusOnline, statuses[offlineMemberGUID])
		assert.Equal(t, repo.GuildMemberStatusOffline, statuses[cutsGUID])

		assert.NotNil(t, cache.guildMembersCache[realmID][cutsGUID])
		assert.NotNil(t, cache.guildMembersCache[realmID][offlineMemberGUID])
	})

	t.Run("throttles source reads", func(t *testing.T) {
		repoMock := &mocks.GuildsRepo{}
		repoMock.On("GuildByRealmAndID", mock.Anything, realmID, guildID).Return(&repo.Guild{
			ID:           guildID,
			GuildMembers: []*repo.GuildMember{{PlayerGUID: leaderGUID, GuildID: guildID}},
		}, nil)

		cache := newSeededCache(repoMock)
		_, err := cache.GuildByRealmAndID(context.Background(), realmID, guildID)
		assert.NoError(t, err)
		_, err = cache.GuildByRealmAndID(context.Background(), realmID, guildID)
		assert.NoError(t, err)

		repoMock.AssertNumberOfCalls(t, "GuildByRealmAndID", 1)
	})

	t.Run("evicts guild deleted in-process", func(t *testing.T) {
		repoMock := &mocks.GuildsRepo{}
		repoMock.On("GuildByRealmAndID", mock.Anything, realmID, guildID).Return(nil, nil)

		cache := newSeededCache(repoMock)
		guild, err := cache.GuildByRealmAndID(context.Background(), realmID, guildID)
		assert.NoError(t, err)
		assert.Nil(t, guild)
		assert.Nil(t, cache.cache[realmID][guildID])
		assert.Nil(t, cache.guildMembersCache[realmID][leaderGUID])
	})

	t.Run("keeps serving stale cache on source error", func(t *testing.T) {
		repoMock := &mocks.GuildsRepo{}
		repoMock.On("GuildByRealmAndID", mock.Anything, realmID, guildID).Return(nil, assert.AnError)

		cache := newSeededCache(repoMock)
		guild, err := cache.GuildByRealmAndID(context.Background(), realmID, guildID)
		assert.NoError(t, err)
		assert.NotNil(t, guild)
		assert.Len(t, guild.GuildMembers, 1)
	})
}

func Test_guildsInMemCache_LoggedInOutTracksNonMembers(t *testing.T) {
	const (
		realmID  = uint32(1)
		charGUID = uint64(42)
	)

	repoMock := &mocks.GuildsRepo{}
	cache := NewGuildsInMemCache(repoMock).(*guildsInMemCache)

	assert.NoError(t, cache.HandleCharacterLoggedIn(events.GWEventCharacterLoggedInPayload{RealmID: realmID, CharGUID: charGUID}))
	_, tracked := cache.onlineChars[realmID][charGUID]
	assert.True(t, tracked)

	assert.NoError(t, cache.HandleCharacterLoggedOut(events.GWEventCharacterLoggedOutPayload{RealmID: realmID, CharGUID: charGUID}))
	_, tracked = cache.onlineChars[realmID][charGUID]
	assert.False(t, tracked)
}

func Test_guildsInMemCache_CreateGuildMarksLeaderOnline(t *testing.T) {
	const (
		realmID    = uint32(1)
		guildID    = uint64(7)
		leaderGUID = uint64(42)
	)

	repoMock := &mocks.GuildsRepo{}
	repoMock.On("CreateGuild", mock.Anything, realmID, "TestGuild", leaderGUID, mock.Anything, mock.Anything).Return(guildID, nil)
	repoMock.On("GuildByRealmAndID", mock.Anything, realmID, guildID).Return(&repo.Guild{
		ID: guildID,
		GuildMembers: []*repo.GuildMember{
			{PlayerGUID: leaderGUID, GuildID: guildID, Status: repo.GuildMemberStatusOffline},
		},
	}, nil)

	cache := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
	cache.cache = map[uint32]map[uint64]*repo.Guild{realmID: {}}
	cache.guildMembersCache = map[uint32]map[uint64]*repo.GuildMember{realmID: {}}

	id, err := cache.CreateGuild(context.Background(), realmID, "TestGuild", leaderGUID, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, guildID, id)
	assert.Equal(t, repo.GuildMemberStatusOnline, cache.guildMembersCache[realmID][leaderGUID].Status)
}

func Test_guildsInMemCache_CreateGuildLeaderStaysOnlineAcrossRefresh(t *testing.T) {
	const (
		realmID    = uint32(1)
		guildID    = uint64(7)
		leaderGUID = uint64(42)
	)

	repoMock := &mocks.GuildsRepo{}
	repoMock.On("CreateGuild", mock.Anything, realmID, "TestGuild", leaderGUID, mock.Anything, mock.Anything).Return(guildID, nil)
	// Both the create hydration and the later refresh read the leader as
	// offline: in cluster mode the world doesn't flush characters.online.
	repoMock.On("GuildByRealmAndID", mock.Anything, realmID, guildID).Return(&repo.Guild{
		ID: guildID,
		GuildMembers: []*repo.GuildMember{
			{PlayerGUID: leaderGUID, GuildID: guildID, Status: repo.GuildMemberStatusOffline, LogoutTime: 10},
		},
	}, nil)

	cache := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
	cache.cache = map[uint32]map[uint64]*repo.Guild{realmID: {}}
	cache.guildMembersCache = map[uint32]map[uint64]*repo.GuildMember{realmID: {}}

	_, err := cache.CreateGuild(context.Background(), realmID, "TestGuild", leaderGUID, nil, nil)
	assert.NoError(t, err)

	guild, err := cache.GuildByRealmAndID(context.Background(), realmID, guildID)
	assert.NoError(t, err)
	assert.Equal(t, repo.GuildMemberStatusOnline, guild.GuildMembers[0].Status)
	repoMock.AssertNumberOfCalls(t, "GuildByRealmAndID", 2)
}

func Test_guildsInMemCache_SeedOnlineChars(t *testing.T) {
	const (
		realmID    = uint32(1)
		guildID    = uint64(64)
		memberGUID = uint64(42)
		otherGUID  = uint64(43)
	)

	repoMock := &mocks.GuildsRepo{}
	cache := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
	member := &repo.GuildMember{PlayerGUID: memberGUID, GuildID: guildID, Status: repo.GuildMemberStatusOffline, LogoutTime: 10}
	cache.cache = map[uint32]map[uint64]*repo.Guild{
		realmID: {guildID: &repo.Guild{ID: guildID, GuildMembers: []*repo.GuildMember{member}}},
	}
	cache.guildMembersCache = map[uint32]map[uint64]*repo.GuildMember{
		realmID: {memberGUID: member},
	}

	cache.SeedOnlineChars(realmID, []uint64{memberGUID, otherGUID})

	assert.Equal(t, repo.GuildMemberStatusOnline, member.Status)
	assert.Equal(t, int64(0), member.LogoutTime)
	_, tracked := cache.onlineChars[realmID][otherGUID]
	assert.True(t, tracked)
}
