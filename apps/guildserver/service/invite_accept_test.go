package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	repoMocks "github.com/walkline/ToCloud9/apps/guildserver/repo/mocks"
	eventsMocks "github.com/walkline/ToCloud9/shared/events/mocks"
)

// sourceAwareRepo wraps a mock so membership looks up FromSource (MySQL path).
type sourceAwareRepo struct {
	*repoMocks.GuildsRepo
	source map[uint64]uint64 // memberGUID -> guildID
}

func (s *sourceAwareRepo) GuildIDByRealmAndMemberGUIDFromSource(_ context.Context, _ uint32, memberGUID uint64) (uint64, error) {
	return s.source[memberGUID], nil
}

func TestInviteAccepted_UsesAtomicAccept(t *testing.T) {
	repoMock := repoMocks.NewGuildsRepo(t)
	guild := &repo.Guild{
		ID: 9,
		GuildRanks: []repo.GuildRank{
			{Rank: 0, Name: "GM"},
			{Rank: 4, Name: "Initiate"},
		},
		GuildMembers: []*repo.GuildMember{{PlayerGUID: 1, Rank: 0, Name: "Lead"}},
	}
	repoMock.On("GuildIDByCharInvite", mock.Anything, uint32(1), uint64(42)).Return(uint64(9), nil)
	repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, uint32(1), uint64(42)).Return(uint64(0), nil).Maybe()
	repoMock.On("GuildByRealmAndID", mock.Anything, uint32(1), uint64(9)).Return(guild, nil)
	repoMock.On("AcceptGuildInvite", mock.Anything, uint32(1), mock.MatchedBy(func(m repo.GuildMember) bool {
		return m.PlayerGUID == 42 && m.Rank == 4
	})).Return(uint64(9), nil)

	producer := &eventsMocks.GuildServiceProducer{}
	producer.On("MemberAdded", mock.Anything).Return(nil)

	src := &sourceAwareRepo{GuildsRepo: repoMock, source: map[uint64]uint64{}}
	svc := NewGuildService(src, producer)

	id, err := svc.InviteAccepted(context.Background(), 1, InviteAcceptedParams{
		CharGUID: 42,
		CharName: "New",
		CharLvl:  10,
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(9), id)
	repoMock.AssertCalled(t, "AcceptGuildInvite", mock.Anything, uint32(1), mock.Anything)
	repoMock.AssertNotCalled(t, "AddGuildMember", mock.Anything, mock.Anything, mock.Anything)
	repoMock.AssertNotCalled(t, "RemoveGuildInviteForCharacter", mock.Anything, mock.Anything, mock.Anything)
}

func TestInviteAccepted_AlreadyInGuildFromSource(t *testing.T) {
	repoMock := repoMocks.NewGuildsRepo(t)
	repoMock.On("GuildIDByCharInvite", mock.Anything, uint32(1), uint64(42)).Return(uint64(9), nil)
	repoMock.On("RemoveGuildInviteForCharacter", mock.Anything, uint32(1), uint64(42)).Return(nil)

	producer := &eventsMocks.GuildServiceProducer{}
	src := &sourceAwareRepo{GuildsRepo: repoMock, source: map[uint64]uint64{42: 3}}
	svc := NewGuildService(src, producer)

	_, err := svc.InviteAccepted(context.Background(), 1, InviteAcceptedParams{CharGUID: 42, CharName: "New"})
	require.ErrorIs(t, err, ErrAlreadyInGuild)
	repoMock.AssertNotCalled(t, "AcceptGuildInvite", mock.Anything, mock.Anything, mock.Anything)
}
