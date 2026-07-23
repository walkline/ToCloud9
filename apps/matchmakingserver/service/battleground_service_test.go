package service

import (
	"sync"
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

func TestAddQueueForGroupMembersIfFreeConcurrent(t *testing.T) {
	s := &battleGroundService{
		playersQueueOrBattleground: make(map[QueuesByRealmAndPlayerKey][]QueueOrBattlegroundLink),
	}
	queue := &GenericBattlegroundQueue{}

	makeGroup := func() *QueuedGroup {
		return &QueuedGroup{
			LeaderGUID: guid.PlayerUnwrapped{RealmID: 1, LowGUID: 1},
			Members:    []guid.PlayerUnwrapped{{RealmID: 1, LowGUID: 2}},
		}
	}

	const attempts = 16
	errs := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.addQueueForGroupMembersIfFree(queue, makeGroup())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	succeeded := 0
	for err := range errs {
		if err == nil {
			succeeded++
		} else {
			assert.ErrorIs(t, err, ErrAlreadyInQueue)
		}
	}
	assert.Equal(t, 1, succeeded, "exactly one concurrent enqueue must win")
}
