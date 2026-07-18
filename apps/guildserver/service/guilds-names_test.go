package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	"github.com/walkline/ToCloud9/apps/guildserver/repo/mocks"
	eventsMocks "github.com/walkline/ToCloud9/shared/events/mocks"
)

func TestGuildNamesByIDs(t *testing.T) {
	const realmID = uint32(1)

	repoMock := &mocks.GuildsRepo{}
	repoMock.On("GuildByRealmAndID", mock.Anything, realmID, uint64(1)).Return(&repo.Guild{ID: 1, Name: "First"}, nil)
	repoMock.On("GuildByRealmAndID", mock.Anything, realmID, uint64(2)).Return(&repo.Guild{ID: 2, Name: "Second"}, nil)
	repoMock.On("GuildByRealmAndID", mock.Anything, realmID, uint64(3)).Return((*repo.Guild)(nil), nil)

	s := NewGuildService(repoMock, &eventsMocks.GuildServiceProducer{})

	names, err := s.GuildNamesByIDs(context.Background(), realmID, []uint64{1, 2, 3, 1})
	assert.NoError(t, err)
	assert.Equal(t, map[uint64]string{1: "First", 2: "Second"}, names)
}
