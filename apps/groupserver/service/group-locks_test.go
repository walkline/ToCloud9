package service

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The race detector fails this test if the lock does not actually serialize
// the goroutines; the final map check proves released locks are dropped.
func TestGroupLocksSerializeSameGroupAndCleanUp(t *testing.T) {
	l := newGroupLocks()

	const goroutines = 32
	counter := 0

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			unlock := l.lock(1, 7)
			counter++
			unlock()
		}()
	}
	close(start)
	wg.Wait()

	assert.Equal(t, goroutines, counter)
	assert.Empty(t, l.locks, "released locks should be dropped from the map")
}

// switchingGroupRepo simulates a player that switches groups between the
// resolution and the lock: the first resolution answers group 1, every
// following one answers group 2.
type switchingGroupRepo struct {
	noopGroupsRepo
	calls int
}

func (r *switchingGroupRepo) GroupIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint, error) {
	r.calls++
	if r.calls == 1 {
		return 1, nil
	}
	return 2, nil
}

func TestGroupsServiceLockGroupOfPlayerRetriesOnGroupSwitch(t *testing.T) {
	repo := &switchingGroupRepo{}
	s := groupServiceImpl{r: repo, locks: newGroupLocks()}

	groupID, unlock, err := s.lockGroupOfPlayer(context.Background(), 1, 42)
	assert.NoError(t, err)
	assert.Equal(t, uint(2), groupID, "should settle on the group confirmed under the lock")
	unlock()

	// 1st resolution (1), re-check (2, mismatch), retry resolution (2), re-check (2).
	assert.Equal(t, 4, repo.calls)
	assert.Empty(t, s.locks.locks)
}
