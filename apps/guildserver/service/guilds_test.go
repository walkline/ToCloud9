package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/apps/guildserver/repo/mocks"
	eventsMocks "github.com/walkline/ToCloud9/shared/events/mocks"
)

func Test_guildServiceImpl_updateRank(t *testing.T) {
	for _, tt := range []struct {
		name            string
		guilds          []repo.Guild
		updater, target uint64
		promote         bool
		expError        bool
		expectedRank    uint8
	}{
		{
			name: "valid promote",
			guilds: []repo.Guild{
				{
					ID: 1,
					GuildRanks: []repo.GuildRank{
						{
							Rank:   0,
							Rights: repo.RightAll,
						},
						{
							Rank:   1,
							Rights: repo.RightEmpty,
						},
						{
							Rank:   2,
							Rights: repo.RightEmpty,
						},
					},
					GuildMembers: []*repo.GuildMember{
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 1,
							Rank:       0,
						},
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 2,
							Rank:       2,
						},
					},
				},
			},
			updater:      1,
			target:       2,
			promote:      true,
			expError:     false,
			expectedRank: 1,
		},
		{
			name: "invalid promote to the same rank",
			guilds: []repo.Guild{
				{
					ID: 1,
					GuildRanks: []repo.GuildRank{
						{
							Rank:   0,
							Rights: repo.RightAll,
						},
						{
							Rank:   1,
							Rights: repo.RightEmpty,
						},
					},
					GuildMembers: []*repo.GuildMember{
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 1,
							Rank:       0,
						},
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 2,
							Rank:       1,
						},
					},
				},
			},
			updater:      1,
			target:       2,
			promote:      true,
			expError:     true,
			expectedRank: 0,
		},
		{
			name: "valid demote",
			guilds: []repo.Guild{
				{
					ID: 1,
					GuildRanks: []repo.GuildRank{
						{
							Rank:   0,
							Rights: repo.RightAll,
						},
						{
							Rank:   1,
							Rights: repo.RightEmpty,
						},
						{
							Rank:   2,
							Rights: repo.RightEmpty,
						},
					},
					GuildMembers: []*repo.GuildMember{
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 1,
							Rank:       0,
						},
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 2,
							Rank:       1,
						},
					},
				},
			},
			updater:      1,
			target:       2,
			promote:      false,
			expError:     false,
			expectedRank: 2,
		},
		{
			name: "invalid demote from the same rank",
			guilds: []repo.Guild{
				{
					ID: 1,
					GuildRanks: []repo.GuildRank{
						{
							Rank:   0,
							Rights: repo.RightAll,
						},
						{
							Rank:   1,
							Rights: repo.RightEmpty,
						},
						{
							Rank:   2,
							Rights: repo.RightEmpty,
						},
					},
					GuildMembers: []*repo.GuildMember{
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 1,
							Rank:       1,
						},
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 2,
							Rank:       1,
						},
					},
				},
			},
			updater:      1,
			target:       2,
			promote:      false,
			expError:     true,
			expectedRank: 0,
		},
		{
			name: "not in the guild",
			guilds: []repo.Guild{
				{
					ID: 1,
					GuildRanks: []repo.GuildRank{
						{
							Rank:   0,
							Rights: repo.RightAll,
						},
						{
							Rank:   1,
							Rights: repo.RightEmpty,
						},
						{
							Rank:   2,
							Rights: repo.RightEmpty,
						},
					},
					GuildMembers: []*repo.GuildMember{
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 1,
							Rank:       1,
						},
					},
				},
			},
			updater:      1,
			target:       2,
			promote:      false,
			expError:     true,
			expectedRank: 0,
		},
		{
			name: "different guilds",
			guilds: []repo.Guild{
				{
					ID: 1,
					GuildRanks: []repo.GuildRank{
						{
							Rank:   0,
							Rights: repo.RightAll,
						},
						{
							Rank:   1,
							Rights: repo.RightEmpty,
						},
						{
							Rank:   2,
							Rights: repo.RightEmpty,
						},
					},
					GuildMembers: []*repo.GuildMember{
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 1,
							Rank:       0,
						},
					},
				},
				{
					ID: 2,
					GuildRanks: []repo.GuildRank{
						{
							Rank:   0,
							Rights: repo.RightAll,
						},
						{
							Rank:   1,
							Rights: repo.RightEmpty,
						},
						{
							Rank:   2,
							Rights: repo.RightEmpty,
						},
					},
					GuildMembers: []*repo.GuildMember{
						&repo.GuildMember{
							GuildID:    2,
							PlayerGUID: 2,
							Rank:       1,
						},
					},
				},
			},
			updater:      1,
			target:       2,
			promote:      false,
			expError:     true,
			expectedRank: 0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repoMock := &mocks.GuildsRepo{}
			repoMock.On(
				"SetMemberRank",
				mock.Anything, mock.Anything,
				mock.Anything, mock.MatchedBy(func(rank uint8) bool { return rank == tt.expectedRank }),
			).Return(nil)

			producerMock := eventsMocks.GuildServiceProducer{}
			producerMock.On("MemberDemote", mock.Anything).Return(nil)
			producerMock.On("MemberPromote", mock.Anything).Return(nil)

			cache := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
			cache.cache = map[uint32]map[uint64]*repo.Guild{1: {}}
			cache.guildMembersCache = map[uint32]map[uint64]*repo.GuildMember{1: {}}

			for _, guild := range tt.guilds {
				guildCpy := guild
				cache.cache[1][guild.ID] = &guildCpy
				for _, member := range guild.GuildMembers {
					cache.guildMembersCache[1][member.PlayerGUID] = member
				}
			}

			g := &guildServiceImpl{
				guildsRepo:     cache,
				eventsProducer: &producerMock,
			}
			if err := g.updateRank(context.Background(), 1, tt.updater, tt.target, tt.promote); (err != nil) != tt.expError {
				t.Errorf("updateRank() error = %v, wantErr %v", err, tt.expError)
			}
			if !tt.expError {
				repoMock.AssertExpectations(t)
			}
		})
	}
}

func Test_guildServiceImpl_Kick(t *testing.T) {
	for _, tt := range []struct {
		name           string
		guilds         []repo.Guild
		kicker, target uint64
		expError       bool
	}{
		{
			name: "valid kick",
			guilds: []repo.Guild{
				{
					ID: 1,
					GuildRanks: []repo.GuildRank{
						{
							Rank:   0,
							Rights: repo.RightAll,
						},
						{
							Rank:   1,
							Rights: repo.RightAll,
						},
					},
					GuildMembers: []*repo.GuildMember{
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 1,
							Rank:       0,
						},
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 2,
							Rank:       1,
						},
					},
				},
			},
			kicker:   1,
			target:   2,
			expError: false,
		},
		{
			name: "invalid kick - rank is lower",
			guilds: []repo.Guild{
				{
					ID: 1,
					GuildRanks: []repo.GuildRank{
						{
							Rank:   0,
							Rights: repo.RightAll,
						},
						{
							Rank:   1,
							Rights: repo.RightEmpty,
						},
					},
					GuildMembers: []*repo.GuildMember{
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 1,
							Rank:       1,
						},
						&repo.GuildMember{
							GuildID:    1,
							PlayerGUID: 2,
							Rank:       0,
						},
					},
				},
			},
			kicker:   1,
			target:   2,
			expError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repoMock := &mocks.GuildsRepo{}
			repoMock.On(
				"RemoveGuildMember",
				mock.Anything, mock.Anything,
				mock.Anything, mock.Anything,
			).Return(nil)

			producerMock := eventsMocks.GuildServiceProducer{}
			producerMock.On("MemberKicked", mock.Anything).Return(nil)

			cache := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
			cache.cache = map[uint32]map[uint64]*repo.Guild{1: {}}
			cache.guildMembersCache = map[uint32]map[uint64]*repo.GuildMember{1: {}}

			for _, guild := range tt.guilds {
				guildCpy := guild
				cache.cache[1][guild.ID] = &guildCpy
				for _, member := range guild.GuildMembers {
					cache.guildMembersCache[1][member.PlayerGUID] = member
				}
			}

			g := &guildServiceImpl{
				guildsRepo:     cache,
				eventsProducer: &producerMock,
			}
			if err := g.Kick(context.Background(), 1, tt.kicker, tt.target); (err != nil) != tt.expError {
				t.Errorf("updateRank() error = %v, wantErr %v", err, tt.expError)
			}
			if !tt.expError {
				repoMock.AssertExpectations(t)
			}
		})
	}
}
