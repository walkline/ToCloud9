package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
