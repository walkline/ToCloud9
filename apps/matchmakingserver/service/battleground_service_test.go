package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/battleground"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

func TestRemoveBattlegroundLinkForPlayer(t *testing.T) {
	// Initialize the service
	service := &battleGroundService{
		playersQueueOrBattleground: make(map[QueuesByRealmAndPlayerKey][]QueueOrBattlegroundLink),
	}

	// Prepare test data
	playerGUID := uint64(12345)
	realmID := uint32(1)
	bgKeyToRemove := BattlegroundKey{InstanceID: 101, RealmID: 1}
	otherBgKey := BattlegroundKey{InstanceID: 102, RealmID: 1}

	service.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		guid.PlayerUnwrapped{
			RealmID: uint16(realmID),
			LowGUID: guid.LowType(playerGUID),
		},
	}] = []QueueOrBattlegroundLink{
		{BattlegroundKey: &bgKeyToRemove, Queue: nil},
		{BattlegroundKey: &otherBgKey, Queue: nil},
	}

	// Call the function to remove the link
	service.removeBattlegroundLinkForPlayer(bgKeyToRemove, playerGUID, realmID)

	// Validate results
	remainingLinks := service.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		guid.PlayerUnwrapped{
			RealmID: uint16(realmID),
			LowGUID: guid.LowType(playerGUID),
		},
	}]
	assert.Len(t, remainingLinks, 1)
	assert.Equal(t, otherBgKey, *remainingLinks[0].BattlegroundKey)
	assert.Equal(t, nil, remainingLinks[0].Queue)
}

func TestRemoveQueueForGroupMembers(t *testing.T) {
	// Initialize the service
	service := &battleGroundService{
		playersQueueOrBattleground: make(map[QueuesByRealmAndPlayerKey][]QueueOrBattlegroundLink),
	}

	// Prepare test data
	player1 := uint64(12345)
	player2 := uint64(67890)
	leader := uint64(11111)
	realmID := uint16(1)
	queueToRemove := &GenericBattlegroundQueue{}
	otherQueue := &GenericBattlegroundQueue{}

	group := &QueuedGroup{
		Members:    []guid.PlayerUnwrapped{{RealmID: realmID, LowGUID: guid.LowType(player1)}, {RealmID: realmID, LowGUID: guid.LowType(player2)}},
		LeaderGUID: guid.PlayerUnwrapped{RealmID: realmID, LowGUID: guid.LowType(leader)},
		RealmID:    uint32(realmID),
	}

	// Populate the map with links
	service.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		guid.PlayerUnwrapped{
			RealmID: realmID,
			LowGUID: guid.LowType(player1),
		},
	}] = []QueueOrBattlegroundLink{
		{Queue: queueToRemove},
		{Queue: otherQueue},
	}

	service.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		guid.PlayerUnwrapped{
			RealmID: realmID,
			LowGUID: guid.LowType(player2),
		},
	}] = []QueueOrBattlegroundLink{
		{Queue: queueToRemove},
	}

	service.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		guid.PlayerUnwrapped{
			RealmID: realmID,
			LowGUID: guid.LowType(leader),
		},
	}] = []QueueOrBattlegroundLink{
		{Queue: queueToRemove},
		{Queue: otherQueue},
	}

	// Call the function
	service.removeQueueForGroupMembers(queueToRemove, group)

	// Validate results
	// Player 1: QueueToRemove removed, OtherQueue remains
	assert.Len(t, service.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		guid.PlayerUnwrapped{
			RealmID: realmID,
			LowGUID: guid.LowType(player1),
		},
	}], 1)
	assert.Equal(t, otherQueue, service.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		guid.PlayerUnwrapped{
			RealmID: realmID,
			LowGUID: guid.LowType(player1),
		},
	}][0].Queue)

	// Player 2: All links removed
	assert.Len(t, service.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		guid.PlayerUnwrapped{
			RealmID: realmID,
			LowGUID: guid.LowType(player2),
		},
	}], 0)

	// Leader: QueueToRemove removed, OtherQueue remains
	assert.Len(t, service.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		guid.PlayerUnwrapped{
			RealmID: realmID,
			LowGUID: guid.LowType(leader),
		},
	}], 1)
	assert.Equal(t, otherQueue, service.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		guid.PlayerUnwrapped{
			RealmID: realmID,
			LowGUID: guid.LowType(leader),
		},
	}][0].Queue)
}

type noopMatchmakingProducer struct{}

func (noopMatchmakingProducer) JoinedQueue(*events.MatchmakingEventPlayersQueuedPayload) error {
	return nil
}
func (noopMatchmakingProducer) InvitedToBGOrArena(*events.MatchmakingEventPlayersInvitedPayload) error {
	return nil
}
func (noopMatchmakingProducer) InviteExpired(*events.MatchmakingEventPlayersInviteExpiredPayload) error {
	return nil
}

func TestExpiredInviteOnlyUnlinksItsBattleground(t *testing.T) {
	bgRepo := repo.NewBattlegroundInMemRepo()
	player := guid.PlayerUnwrapped{RealmID: 1, LowGUID: 7}

	expiredBG := &battleground.Battleground{
		InstanceID:  42,
		RealmID:     1,
		QueueTypeID: battleground.QueueTypeIDWarsongGulch,
		Status:      battleground.StatusWaitJoin,
	}
	expiredBG.InvitedPlayersPerTeam[battleground.TeamAlliance] = []battleground.InvitedPlayer{
		{GUID: player, InvitedTime: time.Now().Add(-2 * time.Minute)},
	}
	activeBG := &battleground.Battleground{
		InstanceID:  43,
		RealmID:     1,
		QueueTypeID: battleground.QueueTypeIDWarsongGulch,
		Status:      battleground.StatusInProgress,
	}
	activeBG.ActivePlayersPerTeam[battleground.TeamAlliance] = []guid.PlayerUnwrapped{player}
	assert.NoError(t, bgRepo.SaveBattleground(context.Background(), expiredBG))
	assert.NoError(t, bgRepo.SaveBattleground(context.Background(), activeBG))

	s := &battleGroundService{
		playersQueueOrBattleground: make(map[QueuesByRealmAndPlayerKey][]QueueOrBattlegroundLink),
		battlegroundsRepo:          bgRepo,
		eventsProducer:             noopMatchmakingProducer{},
	}
	expiredKey := BattlegroundKey{RealmID: 1, InstanceID: 42}
	activeKey := BattlegroundKey{RealmID: 1, InstanceID: 43}
	s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{player}] = []QueueOrBattlegroundLink{
		{BattlegroundKey: &expiredKey},
		{BattlegroundKey: &activeKey},
	}

	s.processExpiredBattlegroundInvitesTick(context.Background())

	links := s.GetQueueOrBattlegroundLinkForPlayer(QueuesByRealmAndPlayerKey{player})
	assert.Len(t, links, 1, "the active battleground link must survive")
	assert.Equal(t, activeKey, *links[0].BattlegroundKey)

	// The invite itself is gone from the expired battleground.
	got, err := bgRepo.GetBattlegroundByInstanceID(context.Background(), 42, repo.RealmWithBattlegroupKey{RealmID: 1})
	assert.NoError(t, err)
	assert.Empty(t, got.InvitedPlayersPerTeam[battleground.TeamAlliance])
}
