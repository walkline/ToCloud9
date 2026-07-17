package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/apps/guildserver/repo/mocks"
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
