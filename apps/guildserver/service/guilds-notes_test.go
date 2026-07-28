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

func notesTestGuild(guildID, updaterGUID, targetGUID uint64) *repo.Guild {
	return &repo.Guild{
		ID: guildID,
		GuildRanks: []repo.GuildRank{
			{Rank: 0, Rights: repo.RightAll},
			{Rank: 4, Rights: repo.RightEmpty},
		},
		GuildMembers: []*repo.GuildMember{
			{PlayerGUID: updaterGUID, GuildID: guildID, Rank: 0, Name: "Updater"},
			{PlayerGUID: targetGUID, GuildID: guildID, Rank: 4, Name: "Target"},
		},
	}
}

// Panicked before the fix: the updater rank was looked up with the guild id
// instead of the updater guid, so the rank was always nil.
func Test_guildServiceImpl_SetMemberOfficerNote(t *testing.T) {
	const (
		realmID     = uint32(1)
		guildID     = uint64(1)
		updaterGUID = uint64(2)
		targetGUID  = uint64(3)
	)

	repoMock := &mocks.GuildsRepo{}
	repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, realmID, updaterGUID).Return(guildID, nil)
	repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, realmID, targetGUID).Return(guildID, nil)
	repoMock.On("GuildByRealmAndID", mock.Anything, realmID, guildID).Return(notesTestGuild(guildID, updaterGUID, targetGUID), nil)
	repoMock.On("SetMemberOfficerNote", mock.Anything, realmID, targetGUID, "note").Return(nil)

	producerMock := eventsMocks.GuildServiceProducer{}
	producerMock.On("MemberOfficerNoteUpdated", mock.Anything).Return(nil)

	service := NewGuildService(repoMock, &producerMock)
	assert.NoError(t, service.SetMemberOfficerNote(context.Background(), realmID, updaterGUID, targetGUID, "note"))
	repoMock.AssertCalled(t, "SetMemberOfficerNote", mock.Anything, realmID, targetGUID, "note")
}

func Test_guildServiceImpl_SetMemberPublicNote(t *testing.T) {
	const (
		realmID     = uint32(1)
		guildID     = uint64(1)
		updaterGUID = uint64(2)
		targetGUID  = uint64(3)
	)

	repoMock := &mocks.GuildsRepo{}
	repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, realmID, updaterGUID).Return(guildID, nil)
	repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, realmID, targetGUID).Return(guildID, nil)
	repoMock.On("GuildByRealmAndID", mock.Anything, realmID, guildID).Return(notesTestGuild(guildID, updaterGUID, targetGUID), nil)
	repoMock.On("SetMemberPublicNote", mock.Anything, realmID, targetGUID, "note").Return(nil)

	producerMock := eventsMocks.GuildServiceProducer{}
	producerMock.On("MemberNoteUpdated", mock.Anything).Return(nil)

	service := NewGuildService(repoMock, &producerMock)
	assert.NoError(t, service.SetMemberPublicNote(context.Background(), realmID, updaterGUID, targetGUID, "note"))
	repoMock.AssertCalled(t, "SetMemberPublicNote", mock.Anything, realmID, targetGUID, "note")
}

// The cached roster can miss a member (mutated outside this service): the
// setters must fail cleanly instead of panicking on the event payload names.
func Test_guildServiceImpl_SetNotes_TargetMissingFromRoster(t *testing.T) {
	const (
		realmID     = uint32(1)
		guildID     = uint64(1)
		updaterGUID = uint64(2)
		targetGUID  = uint64(3)
	)

	guild := notesTestGuild(guildID, updaterGUID, targetGUID)
	guild.GuildMembers = guild.GuildMembers[:1] // target missing from cached roster

	repoMock := &mocks.GuildsRepo{}
	repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, realmID, updaterGUID).Return(guildID, nil)
	repoMock.On("GuildIDByRealmAndMemberGUID", mock.Anything, realmID, targetGUID).Return(guildID, nil)
	repoMock.On("GuildByRealmAndID", mock.Anything, realmID, guildID).Return(guild, nil)

	service := NewGuildService(repoMock, &eventsMocks.GuildServiceProducer{})
	assert.Error(t, service.SetMemberPublicNote(context.Background(), realmID, updaterGUID, targetGUID, "note"))
	assert.Error(t, service.SetMemberOfficerNote(context.Background(), realmID, updaterGUID, targetGUID, "note"))
	repoMock.AssertNotCalled(t, "SetMemberPublicNote", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	repoMock.AssertNotCalled(t, "SetMemberOfficerNote", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
