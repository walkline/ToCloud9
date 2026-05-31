package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"

	redis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type gameServerRedisRepo struct {
	rdb *redis.Client
}

func NewGameServerRedisRepo(rdb *redis.Client) GameServerRepo {
	return &gameServerRedisRepo{rdb: rdb}
}

func (g *gameServerRedisRepo) Upsert(ctx context.Context, server *GameServer) error {
	server.Address = strings.ToLower(server.Address)
	if server.ID == "" {
		server.ID = g.generateID(server.Address)
	}

	key := g.key(server.ID)
	previous, err := g.serverByKey(ctx, key)
	if err != nil {
		return err
	}

	d, err := json.Marshal(server)
	if err != nil {
		return err
	}

	newIndexKey := g.realmIndexKey(server.RealmID, server.IsCrossRealm)
	pipe := g.rdb.TxPipeline()
	pipe.Set(ctx, key, d, 0)
	pipe.SAdd(ctx, newIndexKey, key)
	if previous != nil {
		if oldIndexKey := g.realmIndexKey(previous.RealmID, previous.IsCrossRealm); oldIndexKey != newIndexKey {
			pipe.SRem(ctx, oldIndexKey, key)
		}
	}

	_, err = pipe.Exec(ctx)
	return err
}

func (g *gameServerRedisRepo) Update(ctx context.Context, id string, f func(*GameServer) *GameServer) error {
	oldKey := g.key(id)
	res := g.rdb.Get(ctx, oldKey)
	if res.Err() != nil {
		return res.Err()
	}

	v := &GameServer{}
	err := json.Unmarshal([]byte(res.Val()), v)
	if err != nil {
		return err
	}
	oldRealmID := v.RealmID
	oldIsCrossRealm := v.IsCrossRealm

	newV := f(v)
	d, err := json.Marshal(newV)
	if err != nil {
		return err
	}

	newKey := g.key(newV.ID)
	newIndexKey := g.realmIndexKey(newV.RealmID, newV.IsCrossRealm)
	oldIndexKey := g.realmIndexKey(oldRealmID, oldIsCrossRealm)

	pipe := g.rdb.TxPipeline()
	pipe.Set(ctx, newKey, d, 0)
	pipe.SAdd(ctx, newIndexKey, newKey)
	if oldIndexKey != newIndexKey || oldKey != newKey {
		pipe.SRem(ctx, oldIndexKey, oldKey)
	}
	if oldKey != newKey {
		pipe.Del(ctx, oldKey)
	}

	_, err = pipe.Exec(ctx)
	return err
}

func (g *gameServerRedisRepo) Remove(ctx context.Context, id string) error {
	key := g.key(id)
	res := g.rdb.Get(ctx, key)
	if res.Err() != nil {
		if errors.Is(res.Err(), redis.Nil) {
			return nil
		}

		return res.Err()
	}

	v := &GameServer{}
	err := json.Unmarshal([]byte(res.Val()), v)
	if err != nil {
		return err
	}

	delRes := g.rdb.SRem(ctx, g.realmIndexKey(v.RealmID, v.IsCrossRealm), key)
	if delRes.Err() != nil {
		return delRes.Err()
	}

	delRes = g.rdb.Del(ctx, key)
	return delRes.Err()
}

func (g *gameServerRedisRepo) ListByRealm(ctx context.Context, realmID uint32) ([]GameServer, error) {
	return g.listForRealmOrCrossRealm(ctx, realmID, false)
}

func (g *gameServerRedisRepo) ListOfCrossRealms(ctx context.Context) ([]GameServer, error) {
	return g.listForRealmOrCrossRealm(ctx, 0, true)
}

func (g *gameServerRedisRepo) ListAll(ctx context.Context) ([]GameServer, error) {
	pattern := "ws:*"

	var cursor uint64
	var keys []string

	// Use SCAN to find all keys matching the pattern
	for {
		// Scan with the current cursor value
		var newKeys []string
		var err error
		newKeys, cursor, err = g.rdb.Scan(ctx, cursor, pattern, 10).Result()
		if err != nil {
			return nil, err
		}

		keys = append(keys, newKeys...)

		if cursor == 0 {
			break
		}
	}

	// Retrieve values for all matching keys
	result := make([]GameServer, 0, len(keys))
	for _, key := range keys {
		value, err := g.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		obj := &GameServer{}
		if err := json.Unmarshal([]byte(value), obj); err != nil {
			return nil, err
		}
		result = append(result, *obj)
	}

	return result, nil
}

func (g *gameServerRedisRepo) listForRealmOrCrossRealm(ctx context.Context, realmID uint32, isCrossRealm bool) ([]GameServer, error) {
	res := g.rdb.SMembers(ctx, g.realmIndexKey(realmID, isCrossRealm))
	if res.Err() != nil {
		return nil, res.Err()
	}

	if len(res.Val()) == 0 {
		return []GameServer{}, nil
	}

	mGetRes := g.rdb.MGet(ctx, res.Val()...)
	if mGetRes.Err() != nil {
		return nil, mGetRes.Err()
	}

	resInterface := mGetRes.Val()
	result := make([]GameServer, 0, len(resInterface))
	for i := range resInterface {
		if resInterface[i] == nil {
			log.Warn().Str("key", res.Val()[i]).Msg("fetched nil game server from set")
			continue
		}
		obj := &GameServer{}
		if err := json.Unmarshal([]byte(resInterface[i].(string)), obj); err != nil {
			return nil, err
		}
		result = append(result, *obj)
	}

	return result, nil
}

func (g *gameServerRedisRepo) One(ctx context.Context, id string) (*GameServer, error) {
	return g.serverByKey(ctx, g.key(id))
}

func (g *gameServerRedisRepo) serverByKey(ctx context.Context, key string) (*GameServer, error) {
	getRes := g.rdb.Get(ctx, key)
	if getRes.Err() != nil {
		if errors.Is(getRes.Err(), redis.Nil) {
			return nil, nil
		}
		return nil, getRes.Err()
	}

	resBytes, err := getRes.Bytes()
	if err != nil {
		return nil, err
	}

	obj := &GameServer{}
	if err = json.Unmarshal(resBytes, obj); err != nil {
		return nil, err
	}

	return obj, nil
}

func (g *gameServerRedisRepo) realmIndexKey(realmID uint32, isCrossRealm bool) string {
	if isCrossRealm {
		return "crossrealm:wss"
	}
	return fmt.Sprintf("realm:%d:wss", realmID)
}

func (g *gameServerRedisRepo) key(id string) string {
	return fmt.Sprintf("ws:%s", id)
}

func (g *gameServerRedisRepo) generateID(address string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(address))
	return strconv.FormatUint(uint64(h.Sum32()), 10)
}
