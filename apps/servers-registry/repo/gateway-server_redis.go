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

type gatewayRedisRepo struct {
	rdb *redis.Client
}

func NewGatewayRedisRepo(rdb *redis.Client) GatewayRepo {
	return &gatewayRedisRepo{rdb: rdb}
}

func (g *gatewayRedisRepo) Add(ctx context.Context, server *GatewayServer) (*GatewayServer, error) {
	server.HealthCheckAddr = strings.ToLower(server.HealthCheckAddr)
	server.ID = g.generateID(server.HealthCheckAddr)

	key := g.key(server.ID)
	previous, err := g.gatewayByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	d, err := json.Marshal(server)
	if err != nil {
		return nil, err
	}

	newIndexKey := g.realmIndexKey(server.RealmID)
	pipe := g.rdb.TxPipeline()
	pipe.Set(ctx, key, d, 0)
	pipe.SAdd(ctx, newIndexKey, key)
	if previous != nil {
		if oldIndexKey := g.realmIndexKey(previous.RealmID); oldIndexKey != newIndexKey {
			pipe.SRem(ctx, oldIndexKey, key)
		}
	}

	_, err = pipe.Exec(ctx)
	return server, err
}

func (g *gatewayRedisRepo) Update(ctx context.Context, id string, f func(GatewayServer) GatewayServer) error {
	oldKey := g.key(id)
	res := g.rdb.Get(ctx, oldKey)
	if res.Err() != nil {
		return res.Err()
	}

	v := &GatewayServer{}
	err := json.Unmarshal([]byte(res.Val()), v)
	if err != nil {
		return err
	}
	oldRealmID := v.RealmID

	newV := f(*v)
	d, err := json.Marshal(newV)
	if err != nil {
		return err
	}

	newKey := g.key(newV.ID)
	newIndexKey := g.realmIndexKey(newV.RealmID)
	oldIndexKey := g.realmIndexKey(oldRealmID)

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

func (g *gatewayRedisRepo) Remove(ctx context.Context, healthCheckAddress string) error {
	key := g.key(g.generateID(healthCheckAddress))
	res := g.rdb.Get(ctx, key)
	if res.Err() != nil {
		if errors.Is(res.Err(), redis.Nil) {
			return nil
		}
		return res.Err()
	}

	v := &GatewayServer{}
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

func (g *gatewayRedisRepo) ListByRealm(ctx context.Context, realmID uint32) ([]GatewayServer, error) {
	res := g.rdb.SMembers(ctx, g.realmIndexKey(realmID))
	if res.Err() != nil {
		return nil, res.Err()
	}

	if len(res.Val()) == 0 {
		return []GatewayServer{}, nil
	}

	mGetRes := g.rdb.MGet(ctx, res.Val()...)
	if mGetRes.Err() != nil {
		return nil, mGetRes.Err()
	}

	resInterface := mGetRes.Val()
	result := make([]GatewayServer, 0, len(resInterface))
	for i := range resInterface {
		if resInterface[i] == nil {
			log.Warn().Str("key", res.Val()[i]).Msg("fetched nil gateway from set")
			continue
		}
		obj := &GatewayServer{}
		if err := json.Unmarshal([]byte(resInterface[i].(string)), obj); err != nil {
			return nil, err
		}
		result = append(result, *obj)
	}

	return result, nil
}

func (g *gatewayRedisRepo) gatewayByKey(ctx context.Context, key string) (*GatewayServer, error) {
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

	obj := &GatewayServer{}
	if err = json.Unmarshal(resBytes, obj); err != nil {
		return nil, err
	}

	return obj, nil
}

func (g *gatewayRedisRepo) realmIndexKey(realmID uint32) string {
	return fmt.Sprintf("realm:%d:gws", realmID)
}

func (g *gatewayRedisRepo) key(id string) string {
	return fmt.Sprintf("gw:%s", id)
}

func (g *gatewayRedisRepo) generateID(address string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(address))
	return strconv.FormatUint(uint64(h.Sum32()), 10)
}
