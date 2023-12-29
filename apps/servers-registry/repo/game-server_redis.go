package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
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

	d, err := json.Marshal(server)
	if err != nil {
		return err
	}

	key := g.key(server.ID)
	status := g.rdb.Set(ctx, key, d, 0)
	if status.Err() != nil {
		return status.Err()
	}

	res := g.rdb.SAdd(ctx, g.realmIndexKey(server.RealmID), key)
	if res.Err() != nil {
		g.rdb.Del(ctx, key)
		return res.Err()
	}

	return nil
}

func (g *gameServerRedisRepo) Remove(ctx context.Context, id string) error {
	key := g.key(id)
	res := g.rdb.Get(ctx, key)
	if res.Err() != nil {
		return res.Err()
	}

	v := &GameServer{}
	err := json.Unmarshal([]byte(res.Val()), v)
	if err != nil {
		return err
	}

	delRes := g.rdb.SRem(ctx, g.realmIndexKey(v.RealmID), key)
	if delRes.Err() != nil {
		return delRes.Err()
	}

	delRes = g.rdb.Del(ctx, key)
	return delRes.Err()
}

func (g *gameServerRedisRepo) ListByRealm(ctx context.Context, realmID uint32) ([]GameServer, error) {
	res := g.rdb.SMembers(ctx, g.realmIndexKey(realmID))
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
	getRes := g.rdb.Get(ctx, g.key(id))
	if getRes.Err() != nil {
		if getRes.Err() == redis.Nil {
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

func (g *gameServerRedisRepo) realmIndexKey(realmID uint32) string {
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
