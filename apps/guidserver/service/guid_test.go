package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type MaxGuidStorageMock struct {
	charCounter      []uint64
	itemCounter      []uint64
	instancesCounter []uint64

	charLock      sync.RWMutex
	itemLock      sync.RWMutex
	instancesLock sync.RWMutex

	increaseDelay time.Duration

	counterLock     sync.Mutex
	requestsCounter int
}

func (m *MaxGuidStorageMock) MaxGuidForCharacters(ctx context.Context, realmID uint32) (uint64, error) {
	m.charLock.RLock()
	defer m.charLock.RUnlock()
	return m.charCounter[realmID], nil
}

func (m *MaxGuidStorageMock) MaxGuidForItems(ctx context.Context, realmID uint32) (uint64, error) {
	m.itemLock.RLock()
	defer m.itemLock.RUnlock()

	return m.itemCounter[realmID], nil
}

func (m *MaxGuidStorageMock) MaxGuidForInstances(ctx context.Context, realmID uint32) (uint64, error) {
	m.instancesLock.RLock()
	defer m.instancesLock.RUnlock()

	return m.instancesCounter[realmID], nil
}

func (m *MaxGuidStorageMock) SetMaxGuidForCharacters(ctx context.Context, realmID uint32, value uint64) error {
	m.charLock.Lock()
	defer m.charLock.Unlock()

	m.charCounter[realmID] = value
	return nil
}

func (m *MaxGuidStorageMock) SetMaxGuidForItems(ctx context.Context, realmID uint32, value uint64) error {
	m.itemLock.Lock()
	defer m.itemLock.Unlock()

	m.itemCounter[realmID] = value
	return nil
}

func (m *MaxGuidStorageMock) SetMaxGuidForInstances(ctx context.Context, realmID uint32, value uint64) error {
	m.instancesLock.Lock()
	defer m.instancesLock.Unlock()

	m.instancesCounter[realmID] = value
	return nil
}

func (m *MaxGuidStorageMock) IncreaseMaxGuidForCharacters(ctx context.Context, realmID uint32, increaseAmount uint64) (uint64, error) {
	m.charLock.Lock()
	defer m.charLock.Unlock()

	m.counterLock.Lock()
	m.requestsCounter++
	m.counterLock.Unlock()

	if m.increaseDelay > 0 {
		time.Sleep(m.increaseDelay)
	}

	m.charCounter[realmID] += increaseAmount
	return m.charCounter[realmID], nil
}

func (m *MaxGuidStorageMock) IncreaseMaxGuidForItems(ctx context.Context, realmID uint32, increaseAmount uint64) (uint64, error) {
	m.itemLock.Lock()
	defer m.itemLock.Unlock()

	if m.increaseDelay > 0 {
		time.Sleep(m.increaseDelay)
	}

	m.counterLock.Lock()
	m.requestsCounter++
	m.counterLock.Unlock()

	m.itemCounter[realmID] += increaseAmount
	return m.itemCounter[realmID], nil
}

func (m *MaxGuidStorageMock) IncreaseMaxGuidForInstances(ctx context.Context, realmID uint32, increaseAmount uint64) (uint64, error) {
	m.instancesLock.Lock()
	defer m.instancesLock.Unlock()

	if m.increaseDelay > 0 {
		time.Sleep(m.increaseDelay)
	}

	m.counterLock.Lock()
	m.requestsCounter++
	m.counterLock.Unlock()

	m.instancesCounter[realmID] += increaseAmount
	return m.instancesCounter[realmID], nil
}

func (m *MaxGuidStorageMock) GetRequestsCounter() int {
	m.counterLock.Lock()
	defer m.counterLock.Unlock()
	return m.requestsCounter
}

func Test_guidServiceImpl_GetGuids(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mock := &MaxGuidStorageMock{
		charCounter:      []uint64{1000, 1000, 1000},
		itemCounter:      []uint64{1, 1, 1},
		instancesCounter: []uint64{1, 1, 1},
		//increaseDelay: time.Microsecond * 1,
	}

	s, err := NewGuidService(ctx, nil, mock, []uint32{1, 2}, 4)
	assert.NoError(t, err)
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < 1000; i++ {
				s.GetGuids(ctx, 1, 0, 100)
			}
			wg.Done()
		}()
	}

	wg.Wait()
	//time.Sleep(time.Second)
}

func Test_guidServiceImpl_GetGuids_ThreeTypes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mock := &MaxGuidStorageMock{
		charCounter:      []uint64{1000, 1000, 1000},
		itemCounter:      []uint64{1, 1, 1},
		instancesCounter: []uint64{50, 50, 50},
		increaseDelay:    time.Millisecond * 1,
	}

	expCharDiapasons := []GuidDiapason{{1001, 1001}, {1002, 1002}, {1003, 1003}, {1004, 1004}, {1005, 1005}, {1006, 1006}}
	expItemDiapasons := []GuidDiapason{{2, 2}, {3, 3}, {4, 4}, {5, 5}, {6, 6}, {7, 7}}
	expInstancesDiapasons := []GuidDiapason{{51, 51}, {52, 52}, {53, 53}, {54, 54}, {55, 55}, {56, 56}}

	charDiapasons := []GuidDiapason{}
	itemDiapasons := []GuidDiapason{}
	instancesDiapasons := []GuidDiapason{}

	s, err := NewGuidService(ctx, nil, mock, []uint32{1, 2}, 4)
	assert.NoError(t, err)
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		for i := 0; i < 6; i++ {
			diapason, err := s.GetGuids(ctx, 1, uint8(GuidTypeCharacter), 1)
			assert.NoError(t, err)
			charDiapasons = append(charDiapasons, diapason...)
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < 6; i++ {
			diapason, err := s.GetGuids(ctx, 1, uint8(GuidTypeItem), 1)
			assert.NoError(t, err)
			itemDiapasons = append(itemDiapasons, diapason...)
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < 6; i++ {
			diapason, err := s.GetGuids(ctx, 1, uint8(GuidTypeInstance), 1)
			assert.NoError(t, err)
			instancesDiapasons = append(instancesDiapasons, diapason...)
		}
		wg.Done()
	}()
	wg.Wait()

	assert.Equal(t, expCharDiapasons, charDiapasons)
	assert.Equal(t, expItemDiapasons, itemDiapasons)
	assert.Equal(t, expInstancesDiapasons, instancesDiapasons)
}
