package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/walkline/ToCloud9/apps/chatserver/repo"
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
)

func newTestManager() *ChannelManager {
	return NewChannelManager(repo.NewChannelsNoopRepo())
}

func getOrCreateCustomChannel(t *testing.T, cm *ChannelManager, name string) *ActiveChannel {
	t.Helper()
	ch, err := cm.GetOrCreateChannel(context.Background(), 1, name, 0, pbChat.TeamID_TEAM_ALLIANCE, "", ChannelFlagCustom)
	assert.NoError(t, err)
	return ch
}

// --- ChannelManager tests ---

func TestGetOrCreateChannel_CreatesNew(t *testing.T) {
	cm := newTestManager()
	ch, err := cm.GetOrCreateChannel(context.Background(), 1, "General", 0, pbChat.TeamID_TEAM_ALLIANCE, "", 0)
	assert.NoError(t, err)
	assert.NotNil(t, ch)
	assert.Equal(t, "General", ch.GetName())
}

func TestGetOrCreateChannel_ReturnsSameInstance(t *testing.T) {
	cm := newTestManager()
	ch1, _ := cm.GetOrCreateChannel(context.Background(), 1, "Test", 0, 0, "", ChannelFlagCustom)
	ch2, _ := cm.GetOrCreateChannel(context.Background(), 1, "Test", 0, 0, "", ChannelFlagCustom)
	assert.Same(t, ch1, ch2)
}

func TestGetOrCreateChannel_CaseInsensitiveKey(t *testing.T) {
	cm := newTestManager()
	ch1, _ := cm.GetOrCreateChannel(context.Background(), 1, "Test", 0, 0, "", ChannelFlagCustom)
	ch2, _ := cm.GetOrCreateChannel(context.Background(), 1, "test", 0, 0, "", ChannelFlagCustom)
	assert.Same(t, ch1, ch2)
}

func TestGetOrCreateChannel_DifferentRealmsAreSeparate(t *testing.T) {
	cm := newTestManager()
	ch1, _ := cm.GetOrCreateChannel(context.Background(), 1, "Test", 0, 0, "", ChannelFlagCustom)
	ch2, _ := cm.GetOrCreateChannel(context.Background(), 2, "Test", 0, 0, "", ChannelFlagCustom)
	assert.NotSame(t, ch1, ch2)
}

func TestGetOrCreateChannel_DifferentTeamsAreSeparate(t *testing.T) {
	cm := newTestManager()
	ch1, _ := cm.GetOrCreateChannel(context.Background(), 1, "General", 0, pbChat.TeamID_TEAM_ALLIANCE, "", 0)
	ch2, _ := cm.GetOrCreateChannel(context.Background(), 1, "General", 0, pbChat.TeamID_TEAM_HORDE, "", 0)
	assert.NotSame(t, ch1, ch2)
}

func TestGetChannel_ReturnsNilForUnknown(t *testing.T) {
	cm := newTestManager()
	assert.Nil(t, cm.GetChannel(1, "nonexistent", pbChat.TeamID_TEAM_ALLIANCE))
}

func TestGetChannel_ReturnsExisting(t *testing.T) {
	cm := newTestManager()
	ch, _ := cm.GetOrCreateChannel(context.Background(), 1, "Test", 0, pbChat.TeamID_TEAM_ALLIANCE, "", ChannelFlagCustom)
	found := cm.GetChannel(1, "Test", pbChat.TeamID_TEAM_ALLIANCE)
	assert.Same(t, ch, found)
}

// --- JoinChannel tests ---

func TestJoinChannel_FirstMemberBecomesOwner(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	err := ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "")
	assert.NoError(t, err)
	assert.Equal(t, MemberFlagOwner, ch.GetMemberFlags(100))
}

func TestJoinChannel_SecondMemberNotOwner(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "")
	err := ch.JoinChannel(context.Background(), cm, 1, 200, "Player2", "")
	assert.NoError(t, err)
	assert.Equal(t, MemberFlagNone, ch.GetMemberFlags(200))
}

func TestJoinChannel_PasswordRequired(t *testing.T) {
	cm := newTestManager()
	ch, _ := cm.GetOrCreateChannel(context.Background(), 1, "Secret", 0, pbChat.TeamID_TEAM_ALLIANCE, "pass123", ChannelFlagCustom)

	err := ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "wrong")
	assert.Error(t, err)
	assert.False(t, ch.IsMember(100))

	err = ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "pass123")
	assert.NoError(t, err)
	assert.True(t, ch.IsMember(100))
}

func TestJoinChannel_BannedPlayerRejected(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	ch.mu.Lock()
	ch.bannedUntil[100] = time.Now().Add(time.Hour)
	ch.mu.Unlock()

	err := ch.JoinChannel(context.Background(), cm, 1, 100, "Banned", "")
	assert.Error(t, err)
	assert.False(t, ch.IsMember(100))
}

func TestJoinChannel_ExpiredBanAllowed(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	ch.mu.Lock()
	ch.bannedUntil[100] = time.Now().Add(-time.Hour)
	ch.mu.Unlock()

	err := ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "")
	assert.NoError(t, err)
	assert.True(t, ch.IsMember(100))
}

func TestJoinChannel_AlreadyMemberNoop(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "")
	err := ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "")
	assert.NoError(t, err)
	assert.Equal(t, 1, ch.GetNumMembers())
}

// --- LeaveChannel tests ---

func TestLeaveChannel_RemovesMember(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "")
	_, err := ch.LeaveChannel(context.Background(), cm, 1, 100)
	assert.NoError(t, err)
	assert.False(t, ch.IsMember(100))
	assert.Equal(t, 0, ch.GetNumMembers())
}

func TestLeaveChannel_NotMemberError(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	_, err := ch.LeaveChannel(context.Background(), cm, 1, 999)
	assert.Error(t, err)
}

func TestLeaveChannel_OwnerTransferToModerator(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Owner", "")
	_ = ch.JoinChannel(context.Background(), cm, 1, 200, "Regular", "")
	_ = ch.JoinChannel(context.Background(), cm, 1, 300, "Moderator", "")
	_ = ch.SetModerator(context.Background(), cm, 1, 300, true)

	newOwnerGUID, err := ch.LeaveChannel(context.Background(), cm, 1, 100)
	assert.NoError(t, err)
	assert.Equal(t, uint64(300), newOwnerGUID)

	assert.True(t, ch.GetMemberFlags(300)&MemberFlagOwner != 0)
}

func TestLeaveChannel_OwnerTransferToAnyIfNoModerator(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Owner", "")
	_ = ch.JoinChannel(context.Background(), cm, 1, 200, "Regular", "")

	newOwnerGUID, err := ch.LeaveChannel(context.Background(), cm, 1, 100)
	assert.NoError(t, err)
	assert.Equal(t, uint64(200), newOwnerGUID)

	assert.True(t, ch.GetMemberFlags(200)&MemberFlagOwner != 0)
}

// --- CanSpeak tests ---

func TestCanSpeak_NormalMember(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")
	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "")
	assert.True(t, ch.CanSpeak(100))
}

func TestCanSpeak_NotMember(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")
	assert.False(t, ch.CanSpeak(999))
}

func TestCanSpeak_MutedPlayer(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")
	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "")
	_ = ch.SetMute(context.Background(), cm, 1, 100, true)
	assert.False(t, ch.CanSpeak(100))
}

func TestCanSpeak_ModerationBlocksNormalMember(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")
	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Owner", "")
	_ = ch.JoinChannel(context.Background(), cm, 1, 200, "Regular", "")

	ch.ToggleModeration()

	assert.False(t, ch.CanSpeak(200))
	assert.True(t, ch.CanSpeak(100)) // owner can still speak
}

func TestCanSpeak_ModerationAllowsModerator(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")
	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Owner", "")
	_ = ch.JoinChannel(context.Background(), cm, 1, 200, "Mod", "")
	_ = ch.SetModerator(context.Background(), cm, 1, 200, true)

	ch.ToggleModeration()

	assert.True(t, ch.CanSpeak(200))
}

func TestCanSpeak_ChannelRightsCantSpeak(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	ch.mu.Lock()
	ch.rights = &repo.ChannelRights{Flags: ChannelRightCantSpeak}
	ch.mu.Unlock()

	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "")
	assert.False(t, ch.CanSpeak(100))
}

// --- SetModerator / SetMute tests ---

func TestSetModerator_Success(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")
	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "")

	err := ch.SetModerator(context.Background(), cm, 1, 100, true)
	assert.NoError(t, err)
	assert.True(t, ch.GetMemberFlags(100)&MemberFlagModerator != 0)

	err = ch.SetModerator(context.Background(), cm, 1, 100, false)
	assert.NoError(t, err)
	assert.True(t, ch.GetMemberFlags(100)&MemberFlagModerator == 0)
}

func TestSetModerator_NotMemberError(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	err := ch.SetModerator(context.Background(), cm, 1, 999, true)
	assert.Error(t, err)
}

func TestSetMute_Success(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")
	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "")

	err := ch.SetMute(context.Background(), cm, 1, 100, true)
	assert.NoError(t, err)
	assert.True(t, ch.GetMemberFlags(100)&MemberFlagMuted != 0)

	err = ch.SetMute(context.Background(), cm, 1, 100, false)
	assert.NoError(t, err)
	assert.True(t, ch.GetMemberFlags(100)&MemberFlagMuted == 0)
}

// --- SetOwner tests ---

func TestSetOwner_TransfersOwnership(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")
	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Owner", "")
	_ = ch.JoinChannel(context.Background(), cm, 1, 200, "NewOwner", "")

	err := ch.SetOwner(context.Background(), cm, 1, 200)
	assert.NoError(t, err)

	assert.True(t, ch.GetMemberFlags(200)&MemberFlagOwner != 0)
	assert.True(t, ch.GetMemberFlags(100)&MemberFlagOwner == 0)
}

func TestSetOwner_NotMemberError(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")
	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "Owner", "")

	err := ch.SetOwner(context.Background(), cm, 1, 999)
	assert.Error(t, err)
}

// --- Toggle tests ---

func TestToggleModeration(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	assert.True(t, ch.ToggleModeration())
	assert.False(t, ch.ToggleModeration())
}

func TestToggleAnnouncements(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	// Announce starts as true (from channel creation)
	assert.False(t, ch.ToggleAnnouncements())
	assert.True(t, ch.ToggleAnnouncements())
}

// --- BanPlayer / UnbanPlayer tests ---

func TestBanPlayer_PreventsFutureJoin(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	banTime := time.Now().Add(time.Hour)
	err := cm.BanPlayer(context.Background(), 1, "Test", pbChat.TeamID_TEAM_ALLIANCE, 100, banTime)
	assert.NoError(t, err)

	err = ch.JoinChannel(context.Background(), cm, 1, 100, "Banned", "")
	assert.Error(t, err)
}

func TestUnbanPlayer_AllowsRejoin(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")

	banTime := time.Now().Add(time.Hour)
	_ = cm.BanPlayer(context.Background(), 1, "Test", pbChat.TeamID_TEAM_ALLIANCE, 100, banTime)
	_ = cm.UnbanPlayer(context.Background(), 1, "Test", pbChat.TeamID_TEAM_ALLIANCE, 100)

	err := ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "")
	assert.NoError(t, err)
	assert.True(t, ch.IsMember(100))
}

func TestBanPlayer_UnknownChannelError(t *testing.T) {
	cm := newTestManager()
	err := cm.BanPlayer(context.Background(), 1, "nonexistent", pbChat.TeamID_TEAM_ALLIANCE, 100, time.Now().Add(time.Hour))
	assert.Error(t, err)
}

// --- GetMembers tests ---

func TestGetMembers_ReturnsAllMembers(t *testing.T) {
	cm := newTestManager()
	ch := getOrCreateCustomChannel(t, cm, "Test")
	_ = ch.JoinChannel(context.Background(), cm, 1, 100, "P1", "")
	_ = ch.JoinChannel(context.Background(), cm, 1, 200, "P2", "")
	_ = ch.JoinChannel(context.Background(), cm, 1, 300, "P3", "")

	members := ch.GetMembers()
	assert.Equal(t, 3, len(members))

	guids := make(map[uint64]bool)
	for _, m := range members {
		guids[m.PlayerGUID] = true
	}
	assert.True(t, guids[100])
	assert.True(t, guids[200])
	assert.True(t, guids[300])
}

// --- Ownership disabled tests ---

func TestJoinChannel_NoOwnerWhenOwnershipDisabled(t *testing.T) {
	cm := newTestManager()
	ch, err := cm.GetOrCreateChannel(context.Background(), 1, "noown", 0, pbChat.TeamID_TEAM_ALLIANCE, "", ChannelFlagCustom)
	assert.NoError(t, err)

	ch.mu.Lock()
	ch.ownership = false
	ch.mu.Unlock()

	err = ch.JoinChannel(context.Background(), cm, 1, 100, "Player1", "")
	assert.NoError(t, err)
	assert.Equal(t, MemberFlagNone, ch.GetMemberFlags(100))
}
