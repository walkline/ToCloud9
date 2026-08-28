package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	redis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/shared/events"
)

const (
	accountClaimTimeout = 30 * time.Second
	accountClaimLease   = 35 * time.Second
	accountEvictTimeout = 25 * time.Second
	accountClaimWorkers = 128
)

var (
	commitAccountSessionScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) ~= ARGV[1] then
  return -1
end
local current = redis.call('GET', KEYS[2]) or ''
if current ~= '' and current ~= ARGV[2] and current ~= ARGV[3] then
  return 0
end
redis.call('SET', KEYS[2], ARGV[3])
redis.call('DEL', KEYS[1])
return 1
`)
	releaseAccountSessionScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)
	releaseAccountClaimScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)
)

// AccountSessionCoordinator runs inside the existing servers-registry service.
// It leaves the previous owner authoritative until that gateway confirms full
// teardown, so a failed claimant can never hide an active predecessor.
type AccountSessionCoordinator struct {
	redis    *redis.Client
	nats     *nats.Conn
	gateways Gateway
	// redisRunID is the Redis process identity observed when the coordinator
	// starts. A changed identity means Redis restarted or failed over; claims
	// must remain closed until operators have fenced every gateway.
	redisRunID string

	ctx    context.Context
	cancel context.CancelFunc

	dispatchMu sync.Mutex
	closed     bool
	workers    chan struct{}
	wg         sync.WaitGroup
	claimSub   *nats.Subscription
	releaseSub *nats.Subscription
}

func NewAccountSessionCoordinator(rdb *redis.Client, nc *nats.Conn, gateways Gateway) *AccountSessionCoordinator {
	ctx, cancel := context.WithCancel(context.Background())
	return &AccountSessionCoordinator{
		redis: rdb, nats: nc, gateways: gateways, ctx: ctx, cancel: cancel,
		workers: make(chan struct{}, accountClaimWorkers),
	}
}

func (s *AccountSessionCoordinator) Listen() error {
	var err error
	s.redisRunID, err = redisRunID(s.ctx, s.redis)
	if err != nil {
		return fmt.Errorf("read account session Redis identity: %w", err)
	}
	s.claimSub, err = s.nats.QueueSubscribe(
		events.AccountSessionClaimSubject, "servers-registry-account-sessions",
		func(message *nats.Msg) { s.dispatch(message, s.handleClaim) },
	)
	if err != nil {
		return err
	}
	s.releaseSub, err = s.nats.QueueSubscribe(
		events.AccountSessionReleaseSubject, "servers-registry-account-sessions",
		func(message *nats.Msg) { s.dispatch(message, s.handleRelease) },
	)
	if err != nil {
		_ = s.claimSub.Unsubscribe()
		return err
	}
	return s.nats.Flush()
}

func (s *AccountSessionCoordinator) Close() {
	s.dispatchMu.Lock()
	s.closed = true
	if s.claimSub != nil {
		_ = s.claimSub.Unsubscribe()
	}
	if s.releaseSub != nil {
		_ = s.releaseSub.Unsubscribe()
	}
	s.dispatchMu.Unlock()
	s.cancel()
	s.wg.Wait()
}

func (s *AccountSessionCoordinator) dispatch(message *nats.Msg, handler func(context.Context, events.AccountSessionRequest) error) {
	s.dispatchMu.Lock()
	if s.closed {
		s.dispatchMu.Unlock()
		s.respond(message, errors.New("account session coordinator is stopping"))
		return
	}
	select {
	case s.workers <- struct{}{}:
		s.wg.Add(1)
	default:
		s.dispatchMu.Unlock()
		s.respond(message, errors.New("account session coordinator is busy"))
		return
	}
	s.dispatchMu.Unlock()

	go func() {
		defer s.wg.Done()
		defer func() { <-s.workers }()
		var request events.AccountSessionRequest
		if err := json.Unmarshal(message.Data, &request); err != nil {
			s.respond(message, fmt.Errorf("decode account session request: %w", err))
			return
		}
		ctx, cancel := context.WithTimeout(s.ctx, accountClaimTimeout)
		defer cancel()
		s.respond(message, handler(ctx, request))
	}()
}

func (s *AccountSessionCoordinator) respond(message *nats.Msg, requestErr error) {
	response := events.AccountSessionResponse{Success: requestErr == nil}
	if requestErr != nil {
		response.Error = requestErr.Error()
	}
	payload, err := json.Marshal(response)
	if err != nil {
		return
	}
	if err = message.Respond(payload); err != nil && !errors.Is(err, nats.ErrConnectionClosed) {
		log.Warn().Err(err).Msg("can't reply to account session request")
	}
}

func (s *AccountSessionCoordinator) handleClaim(ctx context.Context, request events.AccountSessionRequest) error {
	if request.AccountID == 0 || request.GatewayID == "" || !validGatewayInstanceID(request.GatewayInstanceID) || request.Token == "" ||
		strings.Contains(request.GatewayID, "|") || strings.Contains(request.GatewayInstanceID, "|") || strings.Contains(request.Token, "|") {
		return errors.New("invalid account session claim")
	}
	active, err := s.gatewayActive(ctx, request.RealmID, request.GatewayID)
	if err != nil {
		return err
	}
	if !active {
		return errors.New("claiming gateway is not registered as active")
	}
	if err = s.ensureRedisContinuity(ctx); err != nil {
		return err
	}
	key := accountSessionKey(request.AccountID)
	pendingKey := key + ":pending"
	owner := formatAccountSessionOwner(request)
	leaseOwner := owner + "|" + strconv.FormatInt(time.Now().UnixNano(), 36)
	for {
		acquired, err := s.redis.SetNX(ctx, pendingKey, leaseOwner, accountClaimLease).Result()
		if err != nil {
			return fmt.Errorf("acquire account session claim: %w", err)
		}
		if acquired {
			break
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	defer s.releasePendingClaim(pendingKey, leaseOwner)

	previous, err := s.redis.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		previous = ""
	} else if err != nil {
		return fmt.Errorf("read account session owner: %w", err)
	}
	if previous != "" && previous != owner {
		previousOwner, err := parseAccountSessionOwner(previous)
		if err != nil {
			return err
		}
		_, evictionErr := s.evictPrevious(ctx, previousOwner)
		if evictionErr != nil {
			// Health-registry absence is not a fencing signal: an isolated gateway
			// can still serve its client. Never replace an owner without a teardown
			// acknowledgement. Orphan cleanup is an explicit operator action after
			// every gateway has been stopped/fenced.
			return evictionErr
		}
	}
	if err = s.ensureRedisContinuity(ctx); err != nil {
		return err
	}
	active, err = s.gatewayActive(ctx, request.RealmID, request.GatewayID)
	if err != nil {
		return err
	}
	if !active {
		return errors.New("claiming gateway became inactive during account session claim")
	}

	result, err := commitAccountSessionScript.Run(
		ctx, s.redis, []string{pendingKey, key}, leaseOwner, previous, owner,
	).Int64()
	if err != nil {
		return fmt.Errorf("commit account session claim: %w", err)
	}
	switch result {
	case 1:
		// Check again after the atomic write. If Redis changed between the
		// pre-commit check and Lua execution, the claimant must not receive a
		// successful response and start serving an unfenced session.
		return s.ensureRedisContinuity(ctx)
	case -1:
		return errors.New("account session claim lease expired")
	default:
		return errors.New("account session ownership changed during claim")
	}
}

func (s *AccountSessionCoordinator) handleRelease(ctx context.Context, request events.AccountSessionRequest) error {
	if request.AccountID == 0 || request.GatewayID == "" || !validGatewayInstanceID(request.GatewayInstanceID) || request.Token == "" ||
		strings.Contains(request.GatewayID, "|") || strings.Contains(request.GatewayInstanceID, "|") || strings.Contains(request.Token, "|") {
		return errors.New("invalid account session release")
	}
	return releaseAccountSessionScript.Run(
		ctx, s.redis, []string{accountSessionKey(request.AccountID)}, formatAccountSessionOwner(request),
	).Err()
}

func (s *AccountSessionCoordinator) evictPrevious(ctx context.Context, previous events.AccountSessionRequest) (bool, error) {
	payload, err := json.Marshal(events.AccountSessionEvictRequest{Token: previous.Token})
	if err != nil {
		return false, err
	}
	evictionCtx, cancel := context.WithTimeout(ctx, accountEvictTimeout)
	defer cancel()
	message, err := s.nats.RequestWithContext(
		evictionCtx, events.AccountSessionEvictSubject(previous.GatewayInstanceID), payload,
	)
	if err != nil {
		return false, fmt.Errorf("request previous account session teardown: %w", err)
	}
	var response events.AccountSessionResponse
	if err = json.Unmarshal(message.Data, &response); err != nil {
		return true, fmt.Errorf("decode account session teardown response: %w", err)
	}
	if !response.Success {
		return true, fmt.Errorf("previous account session teardown was not confirmed: %s", response.Error)
	}
	return true, nil
}

func (s *AccountSessionCoordinator) gatewayActive(ctx context.Context, realmID uint32, gatewayID string) (bool, error) {
	gateways, err := s.gateways.GatewaysForRealm(ctx, realmID)
	if err != nil {
		return false, fmt.Errorf("list gateways for previous account session: %w", err)
	}
	for _, gateway := range gateways {
		if gateway.ID == gatewayID {
			return true, nil
		}
	}
	return false, nil
}

func (s *AccountSessionCoordinator) ensureRedisContinuity(ctx context.Context) error {
	runID, err := redisRunID(ctx, s.redis)
	if err != nil {
		return fmt.Errorf("verify account session Redis identity: %w", err)
	}
	if s.redisRunID == "" || runID != s.redisRunID {
		return errors.New("account session claims are fenced after Redis restart or failover")
	}
	return nil
}

func redisRunID(ctx context.Context, rdb *redis.Client) (string, error) {
	info, err := rdb.Info(ctx, "server").Result()
	if err != nil {
		return "", err
	}
	return parseRedisRunID(info)
}

func parseRedisRunID(info string) (string, error) {
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "run_id:") {
			runID := strings.TrimPrefix(line, "run_id:")
			if runID != "" {
				return runID, nil
			}
		}
	}
	return "", errors.New("Redis INFO did not include run_id")
}

func (s *AccountSessionCoordinator) releasePendingClaim(key, leaseOwner string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := releaseAccountClaimScript.Run(ctx, s.redis, []string{key}, leaseOwner).Err(); err != nil {
		log.Warn().Err(err).Msg("can't release pending account session claim")
	}
}

func accountSessionKey(accountID uint32) string {
	account := strconv.FormatUint(uint64(accountID), 10)
	return "{account-session:" + account + "}:owner"
}

func formatAccountSessionOwner(owner events.AccountSessionRequest) string {
	return strconv.FormatUint(uint64(owner.RealmID), 10) + "|" + owner.GatewayID + "|" + owner.GatewayInstanceID + "|" + owner.Token
}

func validGatewayInstanceID(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func parseAccountSessionOwner(value string) (events.AccountSessionRequest, error) {
	parts := strings.SplitN(value, "|", 4)
	if len(parts) != 4 || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		return events.AccountSessionRequest{}, errors.New("invalid account session owner")
	}
	if !validGatewayInstanceID(parts[2]) {
		return events.AccountSessionRequest{}, errors.New("invalid account session owner instance")
	}
	realmID, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return events.AccountSessionRequest{}, errors.New("invalid account session owner realm")
	}
	return events.AccountSessionRequest{
		RealmID: uint32(realmID), GatewayID: parts[1], GatewayInstanceID: parts[2], Token: parts[3],
	}, nil
}
