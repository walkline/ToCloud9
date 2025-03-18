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

	d, err := json.Marshal(server)
	if err != nil {
		return nil, err
	}

	key := g.key(server.ID)
	status := g.rdb.Set(ctx, key, d, 0)
	if status.Err() != nil {
		return nil, status.Err()
	}

	res := g.rdb.SAdd(ctx, g.realmIndexKey(server.RealmID), key)
	if res.Err() != nil {
		g.rdb.Del(ctx, key)
		return nil, res.Err()
	}

	return server, nil
}

func (g *gatewayRedisRepo) Update(ctx context.Context, id string, f func(GatewayServer) GatewayServer) error {
	res := g.rdb.Get(ctx, g.key(id))
	if res.Err() != nil {
		return res.Err()
	}

	v := &GatewayServer{}
	err := json.Unmarshal([]byte(res.Val()), v)
	if err != nil {
		return err
	}

	newV := f(*v)
	d, err := json.Marshal(newV)
	if err != nil {
		return err
	}

	key := g.key(newV.ID)
	status := g.rdb.Set(ctx, key, d, 0)
	return status.Err()
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
