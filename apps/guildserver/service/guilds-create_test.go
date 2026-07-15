package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/apps/guildserver/repo/mocks"
	eventsMocks "github.com/walkline/ToCloud9/shared/events/mocks"
)

// guildsRepoWithSourceMock makes the repo mock satisfy GuildMembershipSource.
type guildsRepoWithSourceMock struct {
	*mocks.GuildsRepo
	sourceGuildID uint64
	sourceErr     error
}

func (g *guildsRepoWithSourceMock) GuildIDByRealmAndMemberGUIDFromSource(context.Context, uint32, uint64) (uint64, error) {
	return g.sourceGuildID, g.sourceErr
}

func TestGuildServiceCreateGuild(t *testing.T) {
	const (
		realmID    = uint32(1)
		leaderGUID = uint64(42)
	)

	newService := func(repoMock *mocks.GuildsRepo, producerMock *eventsMocks.GuildServiceProducer) GuildService {
		return NewGuildService(repoMock, producerMock)
	}

	t.Run("creates guild with default ranks and publishes event", func(t *testing.T) {
		repoMock := &mocks.GuildsRepo{}
		repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, realmID, leaderGUID).Return(uint64(0), nil)
		repoMock.On(
			"CreateGuild", mock.Anything, realmID, "TestGuild", leaderGUID,
			mock.MatchedBy(func(ranks []repo.GuildRank) bool {
				return len(ranks) == 5 &&
					ranks[0].Rank == uint8(repo.GuildRankGuildMaster) &&
					ranks[0].Rights == uint32(repo.RightAll) &&
					ranks[1].Rank == uint8(repo.GuildRankOfficer) &&
					ranks[1].Rights == uint32(repo.RightAll) &&
					ranks[4].Rank == uint8(repo.GuildRankInitiate)
			}),
		).Return(uint64(7), nil)

		producerMock := &eventsMocks.GuildServiceProducer{}
		producerMock.On("GuildCreated", mock.Anything).Return(nil)

		id, err := newService(repoMock, producerMock).CreateGuild(context.Background(), realmID, leaderGUID, " TestGuild ")
		assert.NoError(t, err)
		assert.Equal(t, uint64(7), id)
		producerMock.AssertCalled(t, "GuildCreated", mock.Anything)
	})

	t.Run("rejects leader already in a guild", func(t *testing.T) {
		repoMock := &mocks.GuildsRepo{}
		repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, realmID, leaderGUID).Return(uint64(3), nil)

		_, err := newService(repoMock, &eventsMocks.GuildServiceProducer{}).CreateGuild(context.Background(), realmID, leaderGUID, "TestGuild")
		assert.ErrorIs(t, err, ErrAlreadyInGuild)
		repoMock.AssertNotCalled(t, "CreateGuild", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("creates guild when positive cached membership is stale in the source", func(t *testing.T) {
		repoMock := &mocks.GuildsRepo{}
		repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, realmID, leaderGUID).Return(uint64(3), nil)
		repoMock.On("CreateGuild", mock.Anything, realmID, "TestGuild", leaderGUID, mock.Anything).Return(uint64(7), nil)

		producerMock := &eventsMocks.GuildServiceProducer{}
		producerMock.On("GuildCreated", mock.Anything).Return(nil)

		repoWithSource := &guildsRepoWithSourceMock{GuildsRepo: repoMock, sourceGuildID: 0}
		id, err := NewGuildService(repoWithSource, producerMock).CreateGuild(context.Background(), realmID, leaderGUID, "TestGuild")
		assert.NoError(t, err)
		assert.Equal(t, uint64(7), id)
	})

	t.Run("rejects leader in a guild when the source confirms membership", func(t *testing.T) {
		repoMock := &mocks.GuildsRepo{}
		repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, realmID, leaderGUID).Return(uint64(3), nil)

		repoWithSource := &guildsRepoWithSourceMock{GuildsRepo: repoMock, sourceGuildID: 3}
		_, err := NewGuildService(repoWithSource, &eventsMocks.GuildServiceProducer{}).CreateGuild(context.Background(), realmID, leaderGUID, "TestGuild")
		assert.ErrorIs(t, err, ErrAlreadyInGuild)
		repoMock.AssertNotCalled(t, "CreateGuild", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("rejects empty and too long names", func(t *testing.T) {
		svc := newService(&mocks.GuildsRepo{}, &eventsMocks.GuildServiceProducer{})

		_, err := svc.CreateGuild(context.Background(), realmID, leaderGUID, "   ")
		assert.ErrorIs(t, err, ErrGuildNameInvalid)

		_, err = svc.CreateGuild(context.Background(), realmID, leaderGUID, strings.Repeat("a", 25))
		assert.ErrorIs(t, err, ErrGuildNameInvalid)
	})

	t.Run("passes name taken error through", func(t *testing.T) {
		repoMock := &mocks.GuildsRepo{}
		repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, realmID, leaderGUID).Return(uint64(0), nil)
		repoMock.On("CreateGuild", mock.Anything, realmID, "TestGuild", leaderGUID, mock.Anything).
			Return(uint64(0), repo.ErrGuildNameTaken)

		_, err := newService(repoMock, &eventsMocks.GuildServiceProducer{}).CreateGuild(context.Background(), realmID, leaderGUID, "TestGuild")
		assert.ErrorIs(t, err, repo.ErrGuildNameTaken)
	})
}
