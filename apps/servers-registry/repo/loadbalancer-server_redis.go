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

type loadBalancerRedisRepo struct {
	rdb *redis.Client
}

func NewLoadBalancerRedisRepo(rdb *redis.Client) LoadBalancerRepo {
	return &loadBalancerRedisRepo{rdb: rdb}
}

func (g *loadBalancerRedisRepo) Add(ctx context.Context, server *LoadBalancerServer) (*LoadBalancerServer, error) {
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

func (g *loadBalancerRedisRepo) Update(ctx context.Context, id string, f func(LoadBalancerServer) LoadBalancerServer) error {
	res := g.rdb.Get(ctx, g.key(id))
	if res.Err() != nil {
		return res.Err()
	}

	v := &LoadBalancerServer{}
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

func (g *loadBalancerRedisRepo) Remove(ctx context.Context, healthCheckAddress string) error {
	key := g.key(g.generateID(healthCheckAddress))
	res := g.rdb.Get(ctx, key)
	if res.Err() != nil {
		return res.Err()
	}

	v := &LoadBalancerServer{}
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

func (g *loadBalancerRedisRepo) ListByRealm(ctx context.Context, realmID uint32) ([]LoadBalancerServer, error) {
	res := g.rdb.SMembers(ctx, g.realmIndexKey(realmID))
	if res.Err() != nil {
		return nil, res.Err()
	}

	if len(res.Val()) == 0 {
		return []LoadBalancerServer{}, nil
	}

	mGetRes := g.rdb.MGet(ctx, res.Val()...)
	if mGetRes.Err() != nil {
		return nil, mGetRes.Err()
	}

	resInterface := mGetRes.Val()
	result := make([]LoadBalancerServer, 0, len(resInterface))
	for i := range resInterface {
		if resInterface[i] == nil {
			log.Warn().Str("key", res.Val()[i]).Msg("fetched nil load balancer from set")
			continue
		}
		obj := &LoadBalancerServer{}
		if err := json.Unmarshal([]byte(resInterface[i].(string)), obj); err != nil {
			return nil, err
		}
		result = append(result, *obj)
	}

	return result, nil
}

func (g *loadBalancerRedisRepo) realmIndexKey(realmID uint32) string {
	return fmt.Sprintf("realm:%d:lbs", realmID)
}

func (g *loadBalancerRedisRepo) key(id string) string {
	return fmt.Sprintf("lb:%s", id)
}

func (g *loadBalancerRedisRepo) generateID(address string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(address))
	return strconv.FormatUint(uint64(h.Sum32()), 10)
}
