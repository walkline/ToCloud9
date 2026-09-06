package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/walkline/ToCloud9/apps/guildserver/repo"
	repoMocks "github.com/walkline/ToCloud9/apps/guildserver/repo/mocks"
	"github.com/walkline/ToCloud9/shared/events"
	eventsMocks "github.com/walkline/ToCloud9/shared/events/mocks"
)

type captureCacheProducer struct {
	payloads []*events.GuildCacheInvalidatePayload
}

func (c *captureCacheProducer) InvalidateGuild(p *events.GuildCacheInvalidatePayload) error {
	cp := *p
	c.payloads = append(c.payloads, &cp)
	return nil
}

func TestGuildsCacheCoherence_PublishesInvalidateOnMutation(t *testing.T) {
	repoMock := repoMocks.NewGuildsRepo(t)
	repoMock.On("SetMessageOfTheDay", mock.Anything, uint32(1), uint64(42), "hi").Return(nil)

	inner := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
	inner.cache[1] = map[uint64]*repo.Guild{42: {ID: 42, MessageOfTheDay: "old"}}

	pub := &captureCacheProducer{}
	cache := NewGuildsCacheWithCoherence(inner, pub, "svc-a")

	err := cache.SetMessageOfTheDay(context.Background(), 1, 42, "hi")
	require.NoError(t, err)
	require.Len(t, pub.payloads, 1)
	assert.Equal(t, "svc-a", pub.payloads[0].ServiceID)
	assert.Equal(t, uint32(1), pub.payloads[0].RealmID)
	assert.Equal(t, uint64(42), pub.payloads[0].GuildID)
}

func TestGuildsCacheCoherence_SkipsSelfEcho(t *testing.T) {
	repoMock := repoMocks.NewGuildsRepo(t)
	inner := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
	inner.SetLocalServiceID("svc-a")

	err := inner.HandleGuildCacheInvalidate(events.GuildCacheInvalidatePayload{
		ServiceID: "svc-a",
		RealmID:   1,
		GuildID:   99,
	})
	require.NoError(t, err)
	repoMock.AssertNotCalled(t, "GuildByRealmAndID", mock.Anything, mock.Anything, mock.Anything)
}

func TestGuildsCacheCoherence_PeerInvalidateMarksDirtyThenReloadsOnRead(t *testing.T) {
	repoMock := repoMocks.NewGuildsRepo(t)
	// Invalidate must not hit MySQL; only the subsequent read does.
	repoMock.On("GuildByRealmAndID", mock.Anything, uint32(1), uint64(7)).
		Return(&repo.Guild{ID: 7, Name: "Reloaded"}, nil).Once()

	inner := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
	inner.SetLocalServiceID("svc-a")
	// Warm entry so we can see dirty-driven reload.
	inner.cache[1] = map[uint64]*repo.Guild{7: {ID: 7, Name: "Stale"}}
	inner.lastRefresh[1] = map[uint64]time.Time{7: time.Now()}

	err := inner.HandleGuildCacheInvalidate(events.GuildCacheInvalidatePayload{
		ServiceID: "svc-b",
		RealmID:   1,
		GuildID:   7,
	})
	require.NoError(t, err)
	repoMock.AssertNotCalled(t, "GuildByRealmAndID", mock.Anything, mock.Anything, mock.Anything)

	g, err := inner.GuildByRealmAndID(context.Background(), 1, 7)
	require.NoError(t, err)
	require.NotNil(t, g)
	assert.Equal(t, "Reloaded", g.Name)
	repoMock.AssertNumberOfCalls(t, "GuildByRealmAndID", 1)
}

type authzFailRepo struct {
	fakeGuildsRepo
	err error
}

func (a *authzFailRepo) MemberAuthzForGuild(context.Context, uint32, uint64, uint64) (*repo.MemberAuthz, error) {
	return nil, a.err
}

func TestBankMemberContext_FailClosedOnAuthzQueryError(t *testing.T) {
	svc := NewGuildBankService(
		&fakeBankRepo{},
		&authzFailRepo{fakeGuildsRepo: fakeGuildsRepo{guild: bankTestGuild()}, err: assert.AnError},
		&eventsMocks.GuildServiceProducer{},
		nil,
	).(*guildBankServiceImpl)

	_, _, _, err := svc.memberContext(context.Background(), 1, bankTestGuild().ID, bankTestGuild().GuildMembers[0].PlayerGUID)
	require.Error(t, err)
}

func TestBankMemberContext_UsesPointAuthz(t *testing.T) {
	svc := NewGuildBankService(
		&fakeBankRepo{},
		&fakeGuildsRepo{guild: bankTestGuild()},
		&eventsMocks.GuildServiceProducer{},
		nil,
	).(*guildBankServiceImpl)

	guild, member, rank, err := svc.memberContext(context.Background(), 1, 7, testOfficerGUID)
	require.NoError(t, err)
	require.NotNil(t, guild)
	assert.Equal(t, uint8(1), member.Rank)
	assert.Equal(t, uint32(repo.RightAll), rank.Rights)
	assert.Equal(t, uint32(5000), rank.MoneyPerDay)
}

func TestGuildsCacheCoherence_MemberNoteUsesSourceGuildID(t *testing.T) {
	repoMock := repoMocks.NewGuildsRepo(t)
	repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, uint32(1), uint64(100)).Return(uint64(55), nil)
	repoMock.On("SetMemberPublicNote", mock.Anything, uint32(1), uint64(100), "n").Return(nil)

	inner := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
	// Cold membership index — force FromSource path.
	pub := &captureCacheProducer{}
	cache := NewGuildsCacheWithCoherence(inner, pub, "svc-a")

	err := cache.SetMemberPublicNote(context.Background(), 1, 100, "n")
	require.NoError(t, err)
	require.Len(t, pub.payloads, 1)
	assert.Equal(t, uint64(55), pub.payloads[0].GuildID)
}

func TestForceRefresh_CoalescesConcurrentLoads(t *testing.T) {
	repoMock := repoMocks.NewGuildsRepo(t)
	// Single slow load shared by waiters.
	repoMock.On("GuildByRealmAndID", mock.Anything, uint32(1), uint64(3)).
		Return(&repo.Guild{ID: 3, Name: "once"}, nil).Once()

	inner := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
	errCh := make(chan error, 2)
	go func() { errCh <- inner.ForceRefreshGuild(context.Background(), 1, 3) }()
	go func() { errCh <- inner.ForceRefreshGuild(context.Background(), 1, 3) }()
	require.NoError(t, <-errCh)
	require.NoError(t, <-errCh)

	g, err := inner.GuildByRealmAndID(context.Background(), 1, 3)
	require.NoError(t, err)
	assert.Equal(t, "once", g.Name)
	repoMock.AssertNumberOfCalls(t, "GuildByRealmAndID", 1)
}

func TestForceRefresh_DoesNotThrottleAfterError(t *testing.T) {
	repoMock := repoMocks.NewGuildsRepo(t)
	// First call fails, second succeeds — ForceRefresh must not be blocked by lastRefresh.
	repoMock.On("GuildByRealmAndID", mock.Anything, uint32(1), uint64(1)).
		Return(nil, assert.AnError).Once()
	repoMock.On("GuildByRealmAndID", mock.Anything, uint32(1), uint64(1)).
		Return(&repo.Guild{ID: 1, Name: "ok"}, nil).Once()

	inner := NewGuildsInMemCache(repoMock).(*guildsInMemCache)
	err := inner.ForceRefreshGuild(context.Background(), 1, 1)
	require.Error(t, err)

	err = inner.ForceRefreshGuild(context.Background(), 1, 1)
	require.NoError(t, err)
	g, err := inner.GuildByRealmAndID(context.Background(), 1, 1)
	require.NoError(t, err)
	assert.Equal(t, "ok", g.Name)
}
