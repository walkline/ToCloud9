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
