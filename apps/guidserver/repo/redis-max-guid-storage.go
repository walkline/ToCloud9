package repo

import (
	"context"
	"errors"
	"fmt"

	redis "github.com/redis/go-redis/v9"
)

type MaxGuidStorage interface {
	MaxGuidProvider

	// SetMaxGuidForCharacters sets max guid for characters. Unsafe fpr concurrent usage. Use IncreaseMaxGuidForCharacters instead.
	SetMaxGuidForCharacters(ctx context.Context, realmID uint32, value uint64) error

	// SetMaxGuidForItems sets max guid for items. Unsafe fpr concurrent usage. Use IncreaseMaxGuidForItems instead.
	SetMaxGuidForItems(ctx context.Context, realmID uint32, value uint64) error

	// IncreaseMaxGuidForCharacters increases max character guid to increaseAmount value and returns new max guid.
	IncreaseMaxGuidForCharacters(ctx context.Context, realmID uint32, increaseAmount uint64) (uint64, error)

	// IncreaseMaxGuidForItems increases max item guid to increaseAmount value and returns new max guid.
	IncreaseMaxGuidForItems(ctx context.Context, realmID uint32, increaseAmount uint64) (uint64, error)
}

// NewRedisMaxGuidStorage returns new redis max guids storage.
func NewRedisMaxGuidStorage(rdb *redis.Client, optimisticLockRetriesCount int) MaxGuidStorage {
	return &redisMaxGuidStorage{
		rdb:          rdb,
		retriesCount: optimisticLockRetriesCount,
	}
}

type redisMaxGuidStorage struct {
	rdb          *redis.Client
	retriesCount int
}

func (r *redisMaxGuidStorage) SetMaxGuidForCharacters(ctx context.Context, realmID uint32, value uint64) error {
	return r.rdb.Set(ctx, r.characterKey(realmID), value, 0).Err()
}

func (r *redisMaxGuidStorage) SetMaxGuidForItems(ctx context.Context, realmID uint32, value uint64) error {
	return r.rdb.Set(ctx, r.itemKey(realmID), value, 0).Err()
}

func (r *redisMaxGuidStorage) MaxGuidForCharacters(ctx context.Context, realmID uint32) (uint64, error) {
	v, err := r.rdb.Get(ctx, r.characterKey(realmID)).Uint64()
	if err != nil && err != redis.Nil {
		return 0, err
	}
	return v, nil
}

func (r *redisMaxGuidStorage) MaxGuidForItems(ctx context.Context, realmID uint32) (uint64, error) {
	v, err := r.rdb.Get(ctx, r.itemKey(realmID)).Uint64()
	if err != nil && err != redis.Nil {
		return 0, err
	}
	return v, nil
}

func (r *redisMaxGuidStorage) IncreaseMaxGuidForCharacters(ctx context.Context, realmID uint32, increaseAmount uint64) (uint64, error) {
	return r.increaseKey(ctx, r.characterKey(realmID), increaseAmount)
}

func (r *redisMaxGuidStorage) IncreaseMaxGuidForItems(ctx context.Context, realmID uint32, increaseAmount uint64) (uint64, error) {
	return r.increaseKey(ctx, r.itemKey(realmID), increaseAmount)
}

func (r *redisMaxGuidStorage) increaseKey(ctx context.Context, key string, increaseAmount uint64) (uint64, error) {
	newMaxAmount := uint64(0)
	txf := func(tx *redis.Tx) error {
		// Get the current value or zero.
		n, err := tx.Get(ctx, key).Uint64()
		if err != nil && err != redis.Nil {
			return err
		}

		// Actual operation (local in optimistic lock).
		newMaxAmount = n + increaseAmount

		// Operation is commited only if the watched keys remain unchanged.
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, key, newMaxAmount, 0)
			return nil
		})
		return err
	}

	// Retry if the key has been changed.
	for i := 0; i < r.retriesCount; i++ {
		err := r.rdb.Watch(ctx, txf, key)
		if err == nil {
			// Success.
			return newMaxAmount, nil
		}
		if err == redis.TxFailedErr {
			// Optimistic lock lost. Retry.
			continue
		}
		// Return any other error.
		return 0, err
	}

	return 0, errors.New("reached maximum number of retries")
}

func (r *redisMaxGuidStorage) itemKey(realmID uint32) string {
	return fmt.Sprintf("realm:%d:maxItem", realmID)
}

func (r *redisMaxGuidStorage) characterKey(realmID uint32) string {
	return fmt.Sprintf("realm:%d:maxChar", realmID)
}
