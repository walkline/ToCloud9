package repo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type layerRedisStore struct{ rdb *redis.Client }

const groupBindingTTL = 24 * time.Hour

func NewLayerRedisStore(rdb *redis.Client) LayerStore { return &layerRedisStore{rdb: rdb} }

func (s *layerRedisStore) Configuration(ctx context.Context, realmID uint32) (map[uint32]uint32, error) {
	value, err := s.rdb.Get(ctx, s.configurationKey(realmID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return map[uint32]uint32{}, nil
	}
	if err != nil {
		return nil, err
	}
	config := map[uint32]uint32{}
	if err := json.Unmarshal(value, &config); err != nil {
		return nil, err
	}
	return config, nil
}

func (s *layerRedisStore) LockRealm(ctx context.Context, realmID uint32) (func(), error) {
	key := fmt.Sprintf("layer:lock:%d", realmID)
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		locked, err := s.rdb.SetNX(ctx, key, token, time.Minute).Result()
		if err != nil {
			return nil, err
		}
		if locked {
			return func() {
				const unlock = `if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) end return 0`
				_ = s.rdb.Eval(context.Background(), unlock, []string{key}, token).Err()
			}, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *layerRedisStore) SetConfiguration(ctx context.Context, realmID uint32, config map[uint32]uint32) error {
	value, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, s.configurationKey(realmID), value, 0).Err()
}

func (s *layerRedisStore) GroupBinding(ctx context.Context, realmID, groupID, mapID uint32) (string, error) {
	key := s.groupKey(realmID, groupID, mapID)
	value, err := s.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err == nil {
		_ = s.rdb.Expire(ctx, key, groupBindingTTL).Err()
	}
	return value, err
}

func (s *layerRedisStore) BindGroup(ctx context.Context, realmID, groupID, mapID uint32, serverID string) (string, error) {
	key := s.groupKey(realmID, groupID, mapID)
	created, err := s.rdb.SetNX(ctx, key, serverID, groupBindingTTL).Result()
	if err != nil {
		return "", err
	}
	if created {
		return serverID, nil
	}
	return s.rdb.Get(ctx, key).Result()
}

func (s *layerRedisStore) SetGroupBinding(ctx context.Context, realmID, groupID, mapID uint32, serverID string) error {
	return s.rdb.Set(ctx, s.groupKey(realmID, groupID, mapID), serverID, groupBindingTTL).Err()
}

func (s *layerRedisStore) ReplaceGroupBinding(ctx context.Context, realmID, groupID, mapID uint32, staleServerID, serverID string) (string, error) {
	const compareAndSet = `
local current = redis.call('GET', KEYS[1])
if current == ARGV[1] then
  redis.call('SET', KEYS[1], ARGV[2], 'PX', ARGV[3])
  return ARGV[2]
end
return current or ''`
	result, err := s.rdb.Eval(ctx, compareAndSet, []string{s.groupKey(realmID, groupID, mapID)}, staleServerID, serverID, groupBindingTTL.Milliseconds()).Text()
	return result, err
}

func (*layerRedisStore) configurationKey(realmID uint32) string {
	return fmt.Sprintf("layer:config:%d", realmID)
}

func (*layerRedisStore) groupKey(realmID, groupID, mapID uint32) string {
	return fmt.Sprintf("layer:group:%d:%d:%d", realmID, groupID, mapID)
}
