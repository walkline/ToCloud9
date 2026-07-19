package service_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/walkline/ToCloud9/apps/matchmakingserver/battleground"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/repo"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/service"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/service/mocks"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

func TestGenericBattlegroundQueue_AddQueuedGroup(t *testing.T) {
	mockService := new(mocks.BattleGroundService)
	mockService.On("BattlegroundsThatNeedPlayers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]battleground.Battleground{}, nil)
	mockService.On("TemplateForQueueTypeID", mock.Anything, mock.Anything).Return(repo.BattlegroundTemplate{}, nil)

	queue := service.NewGenericBattlegroundQueue(mockService, nil, repo.BattlegroundTemplate{TypeID: 1}, 1, 1, 1)
	group := &service.QueuedGroup{
		LeaderGUID:   getGUID(1, 1),
		Members:      []guid.PlayerUnwrapped{getGUID(1, 1)},
		RealmID:      1,
		TeamID:       battleground.TeamAlliance,
		EnqueuedTime: time.Now(),
	}

	err := queue.AddQueuedGroup(group)
	assert.NoError(t, err)
	assert.NotNil(t, queue.QueuedGroupByPlayer(group.LeaderGUID))
}

func TestGenericBattlegroundQueue_CreateBG(t *testing.T) {
	template := repo.BattlegroundTemplate{
		MinPlayersPerTeam: 6,
		MaxPlayersPerTeam: 10,
	}

	mockService := new(mocks.BattleGroundService)
	mockService.On("BattlegroundsThatNeedPlayers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]battleground.Battleground{}, nil)
	mockService.On("TemplateForQueueTypeID", mock.Anything, mock.Anything).Return(template, nil)

	totalAddedHordePlayers := 0
	totalAddedAlliancePlayers := 0

	queue := service.NewGenericBattlegroundQueue(mockService, bgCreatorMock(func(ctx context.Context, template repo.BattlegroundTemplate, queueType battleground.QueueTypeID, bracketID service.BracketID, realmID, battlegroupID uint32, allianceGroups, hordeGroups []service.QueuedGroup) error {
		for _, hordeGroup := range hordeGroups {
			totalAddedHordePlayers += len(hordeGroup.Members)
			totalAddedHordePlayers += 1
		}

		for _, allianceGroup := range allianceGroups {
			totalAddedAlliancePlayers += len(allianceGroup.Members)
			totalAddedAlliancePlayers += 1
		}

		return nil
	}), repo.BattlegroundTemplate{TypeID: 1}, 1, 1, 1)
	assert.NoError(t, queue.AddQueuedGroup(groupWithMembers(2, battleground.TeamAlliance)))
	assert.NoError(t, queue.AddQueuedGroup(groupWithMembers(2, battleground.TeamAlliance)))
	assert.NoError(t, queue.AddQueuedGroup(groupWithMembers(1, battleground.TeamAlliance)))
	assert.NoError(t, queue.AddQueuedGroup(groupWithMembers(1, battleground.TeamAlliance)))
	assert.NoError(t, queue.AddQueuedGroup(groupWithMembers(2, battleground.TeamHorde)))
	assert.NoError(t, queue.AddQueuedGroup(groupWithMembers(2, battleground.TeamHorde)))
	assert.NoError(t, queue.AddQueuedGroup(groupWithMembers(2, battleground.TeamHorde)))

	mockService.AssertExpectations(t)

	assert.Equal(t, int(template.MinPlayersPerTeam), totalAddedHordePlayers)
	assert.Equal(t, int(template.MinPlayersPerTeam), totalAddedAlliancePlayers)
}

// TestGenericBattlegroundQueue_FillInExistingBG in this test we have existing bg with
// Maximum 3 players per team.
// For alliance there are 3 players.
// For horde - 2.
// Should invite group with 1 horde player.
func TestGenericBattlegroundQueue_FillInExistingBG(t *testing.T) {
	template := repo.BattlegroundTemplate{
		MinPlayersPerTeam: 2,
		MaxPlayersPerTeam: 3,
	}

	groupThatShouldBeInvited := groupWithMembers(1, battleground.TeamHorde)

	mockService := new(mocks.BattleGroundService)
	mockService.On("BattlegroundsThatNeedPlayers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]battleground.Battleground{
		{
			MinPlayersPerTeam: template.MinPlayersPerTeam,
			MaxPlayersPerTeam: template.MaxPlayersPerTeam,
			ActivePlayersPerTeam: [3][]guid.PlayerUnwrapped{
				{},
				{ // Alliance
					getGUID(1, 1),
					getGUID(1, 2),
					getGUID(1, 3),
				},
				{ // Horde
					getGUID(1, 4),
					getGUID(1, 5),
				},
			},
			InvitedPlayersPerTeam: [3][]battleground.InvitedPlayer{},
		},
	}, nil)
	mockService.On("TemplateForQueueTypeID", mock.Anything, mock.Anything).Return(template, nil)
	mockService.On(
		"InviteGroups",
		mock.Anything,
		mock.MatchedBy(func(v interface{}) bool {
			return v.([]service.QueuedGroup)[0].LeaderGUID == groupThatShouldBeInvited.LeaderGUID
		}),
		mock.Anything,
		mock.Anything,
	).Return(nil)

	queue := service.NewGenericBattlegroundQueue(mockService, nil, repo.BattlegroundTemplate{TypeID: 1}, 1, 1, 1)

	// Shouldn't be invited, since enough alliance players
	assert.NoError(t, queue.AddQueuedGroup(groupWithMembers(2, battleground.TeamAlliance)))

	// Should invite, since 1 place for horde
	assert.NoError(t, queue.AddQueuedGroup(groupThatShouldBeInvited))

	mockService.AssertExpectations(t)
}

// TestGenericBattlegroundQueue_BalancedBGDoesNotAbsorbNewGroups reproduces the
// 10v5 bug: a balanced in-progress battleground (5v5, max 10) must not absorb a
// fresh 5-player group on one team; the group stays queued and pops a new
// instance once the opposite faction is available.
func TestGenericBattlegroundQueue_BalancedBGDoesNotAbsorbNewGroups(t *testing.T) {
	template := repo.BattlegroundTemplate{
		MinPlayersPerTeam: 5,
		MaxPlayersPerTeam: 10,
	}

	runningBG := battleground.Battleground{
		MinPlayersPerTeam: template.MinPlayersPerTeam,
		MaxPlayersPerTeam: template.MaxPlayersPerTeam,
	}
	for i := uint32(1); i <= 5; i++ {
		runningBG.ActivePlayersPerTeam[battleground.TeamAlliance] = append(runningBG.ActivePlayersPerTeam[battleground.TeamAlliance], getGUID(1, i))
		runningBG.ActivePlayersPerTeam[battleground.TeamHorde] = append(runningBG.ActivePlayersPerTeam[battleground.TeamHorde], getGUID(1, i+100))
	}

	mockService := new(mocks.BattleGroundService)
	mockService.On("BattlegroundsThatNeedPlayers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]battleground.Battleground{runningBG}, nil)
	mockService.On("TemplateForQueueTypeID", mock.Anything, mock.Anything).Return(template, nil)
	// No InviteGroups expectation: inviting into the running BG must not happen.

	newBGCreated := false
	queue := service.NewGenericBattlegroundQueue(mockService, bgCreatorMock(func(ctx context.Context, template repo.BattlegroundTemplate, queueType battleground.QueueTypeID, bracketID service.BracketID, realmID, battlegroupID uint32, allianceGroups, hordeGroups []service.QueuedGroup) error {
		newBGCreated = true
		return nil
	}), repo.BattlegroundTemplate{TypeID: 1}, 1, 1, 1)

	// 5-player alliance group queues while the balanced match runs.
	assert.NoError(t, queue.AddQueuedGroup(groupWithMembers(5, battleground.TeamAlliance)))
	assert.False(t, newBGCreated, "no horde in queue yet")
	assert.Len(t, queue.GetAllQueuedGroups(), 1, "group must stay in queue")

	// Horde players arrive (e.g. the bot fill): a NEW instance pops.
	for i := 0; i < 5; i++ {
		assert.NoError(t, queue.AddQueuedGroup(groupWithMembers(1, battleground.TeamHorde)))
	}
	assert.True(t, newBGCreated, "second instance must be created")

	mockService.AssertExpectations(t)
}

// TestGenericBattlegroundQueue_BackfillCountsLeaders ensures backfill sizing
// counts the leader (Members excludes it): a single free slot must invite
// exactly one solo group, not every solo group in the queue.
func TestGenericBattlegroundQueue_BackfillCountsLeaders(t *testing.T) {
	template := repo.BattlegroundTemplate{
		MinPlayersPerTeam: 5,
		MaxPlayersPerTeam: 10,
	}

	runningBG := battleground.Battleground{
		MinPlayersPerTeam: template.MinPlayersPerTeam,
		MaxPlayersPerTeam: template.MaxPlayersPerTeam,
	}
	for i := uint32(1); i <= 5; i++ {
		if i <= 4 {
			runningBG.ActivePlayersPerTeam[battleground.TeamAlliance] = append(runningBG.ActivePlayersPerTeam[battleground.TeamAlliance], getGUID(1, i))
		}
		runningBG.ActivePlayersPerTeam[battleground.TeamHorde] = append(runningBG.ActivePlayersPerTeam[battleground.TeamHorde], getGUID(1, i+100))
	}

	mockService := new(mocks.BattleGroundService)
	// Let the first five groups queue up without a battleground to fill.
	mockService.On("BattlegroundsThatNeedPlayers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]battleground.Battleground{}, nil).Times(5)
	// The sixth pass sees the 4v5 battleground: one free alliance slot.
	mockService.On("BattlegroundsThatNeedPlayers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]battleground.Battleground{runningBG}, nil)
	mockService.On("TemplateForQueueTypeID", mock.Anything, mock.Anything).Return(template, nil)

	var invited []service.QueuedGroup
	mockService.On("InviteGroups", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		invited = append(invited, args.Get(1).([]service.QueuedGroup)...)
	}).Return(nil)

	queue := service.NewGenericBattlegroundQueue(mockService, nil, repo.BattlegroundTemplate{TypeID: 1}, 1, 1, 1)

	for i := 0; i < 6; i++ {
		assert.NoError(t, queue.AddQueuedGroup(groupWithMembers(1, battleground.TeamAlliance)))
	}

	players := 0
	for _, g := range invited {
		players += len(g.Members) + 1
	}
	assert.Equal(t, 1, players, "one free slot must invite exactly one player")
	assert.Len(t, queue.GetAllQueuedGroups(), 5, "remaining solo groups must stay queued")
}

func TestGenericBattlegroundQueue_RemoveQueuedGroup(t *testing.T) {
	mockService := new(mocks.BattleGroundService)
	mockService.On("BattlegroundsThatNeedPlayers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]battleground.Battleground{}, nil)
	mockService.On("TemplateForQueueTypeID", mock.Anything, mock.Anything).Return(repo.BattlegroundTemplate{}, nil)

	queue := service.NewGenericBattlegroundQueue(mockService, nil, repo.BattlegroundTemplate{TypeID: 1}, 1, 1, 1)

	group := &service.QueuedGroup{
		LeaderGUID:   getGUID(1, 1),
		Members:      []guid.PlayerUnwrapped{getGUID(1, 1)},
		RealmID:      1,
		TeamID:       battleground.TeamAlliance,
		EnqueuedTime: time.Now(),
	}

	queue.AddQueuedGroup(group)
	err := queue.RemoveQueuedGroup(group.LeaderGUID)
	assert.NoError(t, err)
	assert.Nil(t, queue.QueuedGroupByPlayer(group.LeaderGUID))
}

func TestGenericBattlegroundQueue_RemoveQueuedGroup_NotFound(t *testing.T) {
	mockService := new(mocks.BattleGroundService)
	mockService.On("BattlegroundsThatNeedPlayers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]battleground.Battleground{}, nil)
	mockService.On("TemplateForQueueTypeID", mock.Anything, mock.Anything).Return(repo.BattlegroundTemplate{}, nil)

	queue := service.NewGenericBattlegroundQueue(mockService, nil, repo.BattlegroundTemplate{TypeID: 1}, 1, 1, 1)

	err := queue.RemoveQueuedGroup(getGUID(1, 999))
	assert.ErrorIs(t, err, service.ErrPlayerNotFound)
}

func TestGenericBattlegroundQueue_GetAllQueuedGroups(t *testing.T) {
	mockService := new(mocks.BattleGroundService)
	mockService.On("BattlegroundsThatNeedPlayers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]battleground.Battleground{}, nil)
	mockService.On("TemplateForQueueTypeID", mock.Anything, mock.Anything).Return(repo.BattlegroundTemplate{}, nil)

	queue := service.NewGenericBattlegroundQueue(mockService, nil, repo.BattlegroundTemplate{TypeID: 1}, 1, 1, 1)

	group1 := &service.QueuedGroup{LeaderGUID: getGUID(1, 1), Members: []guid.PlayerUnwrapped{getGUID(1, 1)}, RealmID: 1, TeamID: battleground.TeamAlliance, EnqueuedTime: time.Now()}
	group2 := &service.QueuedGroup{LeaderGUID: getGUID(1, 2), Members: []guid.PlayerUnwrapped{getGUID(1, 2)}, RealmID: 1, TeamID: battleground.TeamHorde, EnqueuedTime: time.Now()}

	queue.AddQueuedGroup(group1)
	queue.AddQueuedGroup(group2)

	groups := queue.GetAllQueuedGroups()
	assert.Len(t, groups, 2)
}

func getGUID(realmID uint16, low uint32) guid.PlayerUnwrapped {
	return guid.PlayerUnwrapped{
		RealmID: realmID,
		LowGUID: guid.LowType(low),
	}
}

func groupWithMembers(membersCount int, team battleground.PVPTeam) *service.QueuedGroup {
	members := make([]guid.PlayerUnwrapped, 0, membersCount)
	for i := 0; i < membersCount-1; i++ {
		members = append(members, guid.PlayerUnwrapped{
			RealmID: 1,
			LowGUID: guid.LowType(rand.Uint32()),
		})
	}
	return &service.QueuedGroup{
		LeaderGUID: guid.PlayerUnwrapped{
			RealmID: 1,
			LowGUID: guid.LowType(rand.Uint32()),
		},
		Members: members,
		TeamID:  team,
	}
}

type bgCreatorMock func(
	ctx context.Context,
	template repo.BattlegroundTemplate,
	queueType battleground.QueueTypeID,
	bracketID service.BracketID,
	realmID, battlegroupID uint32,
	allianceGroups, hordeGroups []service.QueuedGroup,
) error

func (f bgCreatorMock) CreateBattleground(
	ctx context.Context,
	template repo.BattlegroundTemplate,
	queueType battleground.QueueTypeID,
	bracketID service.BracketID,
	realmID, battlegroupID uint32,
	allianceGroups, hordeGroups []service.QueuedGroup,
) error {
	return f(ctx, template, queueType, bracketID, realmID, battlegroupID, allianceGroups, hordeGroups)
}
