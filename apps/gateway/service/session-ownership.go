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
	"github.com/rs/zerolog"
)

const (
	sessionEvictionSubjectPrefix = "tc9.gateway.session.evict."
	evictionWorkerCount          = 32
	evictionStreamMaxLength      = 4096
	evictionAcknowledgeTimeout   = 4 * time.Second
)

var (
	ErrSessionOwnershipSuperseded = errors.New("session ownership was superseded by another login")

	// The owner key and the previous gateway's stream use the same realm hash
	// tag. The compare, replacement and durable eviction event therefore remain
	// atomic on both standalone Redis and Redis Cluster.
	claimSessionOwnershipScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1]) or ''
if current ~= ARGV[1] then
  return {0, current}
end
redis.call('SET', KEYS[1], ARGV[2])
if current ~= '' and current ~= ARGV[2] then
  redis.call('XADD', KEYS[2], 'MAXLEN', '~', ARGV[5], '*',
    'token', ARGV[3], 'ack_key', ARGV[4])
end
return {1, current}
`)

	releaseSessionOwnershipScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)
)

type sessionEvictionRequest struct {
	Token  string `json:"token"`
	AckKey string `json:"ack_key"`
}

// SessionOwnershipService stores durable, token-fenced ownership in Redis.
// Takeovers are delivered through both NATS (fast path) and a Redis Stream
// (durable path). Redis traffic while players are idle is one heartbeat per
// gateway, independent of the number of connected players.
type SessionOwnershipService struct {
	redis       *redis.Client
	nats        *nats.Conn
	logger      *zerolog.Logger
	gatewayID   string
	realmID     uint32
	livenessTTL time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.RWMutex
	sessions map[string]func(context.Context)
	workers  chan struct{}
	wg       sync.WaitGroup

	evictionMu         sync.Mutex
	processedEvictions map[string]time.Time
}

func NewSessionOwnershipService(rdb *redis.Client, nc *nats.Conn, logger *zerolog.Logger, gatewayID string, realmID uint32, livenessTTL time.Duration) *SessionOwnershipService {
	ctx, cancel := context.WithCancel(context.Background())
	return &SessionOwnershipService{
		redis: rdb, nats: nc, logger: logger, gatewayID: gatewayID, realmID: realmID,
		livenessTTL: livenessTTL, ctx: ctx, cancel: cancel,
		sessions: make(map[string]func(context.Context)), workers: make(chan struct{}, evictionWorkerCount),
		processedEvictions: make(map[string]time.Time),
	}
}

func (s *SessionOwnershipService) Listen() error {
	if err := s.writeHeartbeat(s.ctx); err != nil {
		return fmt.Errorf("write initial gateway session heartbeat: %w", err)
	}
	_, err := s.nats.Subscribe(sessionEvictionSubjectPrefix+s.gatewayID, func(message *nats.Msg) {
		var request sessionEvictionRequest
		if err := json.Unmarshal(message.Data, &request); err != nil {
			s.logger.Error().Err(err).Msg("can't decode session eviction request")
			return
		}
		// Do not block the NATS subscription when all workers are busy. The
		// durable Redis stream remains the retry path.
		_ = s.tryDispatchEviction(request)
	})
	if err != nil {
		return err
	}
	if err = s.nats.Flush(); err != nil {
		return err
	}
	s.wg.Add(2)
	go s.runHeartbeat()
	go s.consumeEvictionStream()
	return nil
}

func (s *SessionOwnershipService) Close() {
	s.cancel()
	s.wg.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.redis.Del(ctx, s.gatewayLivenessKey(s.gatewayID)).Err(); err != nil && !errors.Is(err, context.Canceled) {
		s.logger.Warn().Err(err).Msg("can't remove gateway session heartbeat")
	}
}

func (s *SessionOwnershipService) Register(token string, evict func(context.Context)) func() {
	s.mu.Lock()
	s.sessions[token] = evict
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
	}
}

func (s *SessionOwnershipService) ClaimCharacter(ctx context.Context, characterGUID uint64, token string) error {
	return s.claim(ctx, s.characterKey(characterGUID), token)
}

func (s *SessionOwnershipService) ReleaseCharacter(ctx context.Context, characterGUID uint64, token string) error {
	return s.release(ctx, s.characterKey(characterGUID), token)
}

func (s *SessionOwnershipService) claim(ctx context.Context, key, token string) error {
	owner := s.owner(token)
	for attempts := 0; attempts < 8; attempts++ {
		previous, err := s.redis.Get(ctx, key).Result()
		if errors.Is(err, redis.Nil) {
			previous = ""
		} else if err != nil {
			return fmt.Errorf("read session ownership: %w", err)
		}
		if previous == owner {
			return nil
		}

		previousGateway, previousToken, validPrevious := parseSessionOwner(previous)
		streamKey := s.evictionStreamKey(s.gatewayID)
		if validPrevious {
			streamKey = s.evictionStreamKey(previousGateway)
		}
		ackKey := s.evictionAckKey(token, attempts)
		result, err := claimSessionOwnershipScript.Run(
			ctx, s.redis, []string{key, streamKey}, previous, owner,
			previousToken, ackKey, evictionStreamMaxLength,
		).Slice()
		if err != nil {
			return fmt.Errorf("claim session ownership: %w", err)
		}
		claimed, err := scriptInt64(result, 0)
		if err != nil {
			return err
		}
		if claimed == 0 {
			continue
		}

		if validPrevious && previous != owner {
			s.publishFastEviction(previousGateway, sessionEvictionRequest{Token: previousToken, AckKey: ackKey})
			s.waitForEvictionAcknowledgement(ctx, previousGateway, ackKey)
		}
		current, err := s.redis.Get(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("verify session ownership: %w", err)
		}
		if current != owner {
			return ErrSessionOwnershipSuperseded
		}
		return nil
	}
	return errors.New("session ownership changed too frequently")
}

func (s *SessionOwnershipService) publishFastEviction(gatewayID string, request sessionEvictionRequest) {
	payload, err := json.Marshal(request)
	if err != nil {
		return
	}
	if err = s.nats.Publish(sessionEvictionSubjectPrefix+gatewayID, payload); err != nil {
		s.logger.Warn().Err(err).Str("gatewayID", gatewayID).Msg("can't publish fast session eviction")
	}
}

func (s *SessionOwnershipService) waitForEvictionAcknowledgement(ctx context.Context, gatewayID, ackKey string) {
	alive, err := s.redis.Exists(ctx, s.gatewayLivenessKey(gatewayID)).Result()
	if err != nil || alive == 0 {
		return
	}
	waitCtx, cancel := context.WithTimeout(ctx, evictionAcknowledgeTimeout)
	defer cancel()
	_, _ = s.redis.BLPop(waitCtx, evictionAcknowledgeTimeout, ackKey).Result()
}

func (s *SessionOwnershipService) tryDispatchEviction(request sessionEvictionRequest) bool {
	select {
	case s.workers <- struct{}{}:
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() { <-s.workers }()
			s.processEviction(request)
		}()
		return true
	default:
		return false
	}
}

func (s *SessionOwnershipService) processEviction(request sessionEvictionRequest) {
	evictionID := request.AckKey
	if evictionID == "" {
		evictionID = request.Token
	}
	s.evictionMu.Lock()
	now := time.Now()
	if expiresAt, exists := s.processedEvictions[evictionID]; exists && expiresAt.After(now) {
		s.evictionMu.Unlock()
		return
	}
	if len(s.processedEvictions) >= evictionStreamMaxLength {
		for id, expiresAt := range s.processedEvictions {
			if !expiresAt.After(now) {
				delete(s.processedEvictions, id)
			}
		}
		if len(s.processedEvictions) >= evictionStreamMaxLength {
			for id := range s.processedEvictions {
				delete(s.processedEvictions, id)
				break
			}
		}
	}
	s.processedEvictions[evictionID] = now.Add(time.Minute)
	s.evictionMu.Unlock()

	s.mu.RLock()
	evict := s.sessions[request.Token]
	s.mu.RUnlock()
	if evict != nil {
		s.logger.Info().Msg("evicting a superseded local gateway session")
		ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
		evict(ctx)
		cancel()
	}
	if request.AckKey != "" {
		pipe := s.redis.Pipeline()
		pipe.LPush(s.ctx, request.AckKey, "1")
		pipe.Expire(s.ctx, request.AckKey, 30*time.Second)
		if _, err := pipe.Exec(s.ctx); err != nil && s.ctx.Err() == nil {
			s.logger.Warn().Err(err).Msg("can't acknowledge session eviction")
		}
	}
}

func (s *SessionOwnershipService) consumeEvictionStream() {
	defer s.wg.Done()
	stream := s.evictionStreamKey(s.gatewayID)
	lastID := "0-0"
	for s.ctx.Err() == nil {
		result, err := s.redis.XRead(s.ctx, &redis.XReadArgs{
			Streams: []string{stream, lastID}, Count: 64, Block: 2 * time.Second,
		}).Result()
		if err != nil {
			if !errors.Is(err, redis.Nil) && s.ctx.Err() == nil {
				s.logger.Warn().Err(err).Msg("can't consume durable session evictions")
				time.Sleep(time.Second)
			}
			continue
		}
		for _, streamResult := range result {
			for _, message := range streamResult.Messages {
				lastID = message.ID
				request := sessionEvictionRequest{
					Token: stringValue(message.Values["token"]), AckKey: stringValue(message.Values["ack_key"]),
				}
				for !s.tryDispatchEviction(request) && s.ctx.Err() == nil {
					time.Sleep(10 * time.Millisecond)
				}
			}
		}
	}
}

func (s *SessionOwnershipService) runHeartbeat() {
	defer s.wg.Done()
	interval := s.livenessTTL / 3
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.writeHeartbeat(s.ctx); err != nil && s.ctx.Err() == nil {
				s.logger.Warn().Err(err).Msg("can't refresh gateway session heartbeat")
			}
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *SessionOwnershipService) writeHeartbeat(ctx context.Context) error {
	return s.redis.Set(ctx, s.gatewayLivenessKey(s.gatewayID), "1", s.livenessTTL).Err()
}

func (s *SessionOwnershipService) release(ctx context.Context, key, token string) error {
	return releaseSessionOwnershipScript.Run(ctx, s.redis, []string{key}, s.owner(token)).Err()
}

func (s *SessionOwnershipService) owner(token string) string {
	return s.gatewayID + "|" + token
}

func parseSessionOwner(value string) (string, string, bool) {
	parts := strings.SplitN(value, "|", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (s *SessionOwnershipService) realmHashTag() string {
	return "{gateway-session:" + strconv.FormatUint(uint64(s.realmID), 10) + "}"
}

func (s *SessionOwnershipService) characterKey(characterGUID uint64) string {
	return s.realmHashTag() + ":owner:character:" + strconv.FormatUint(characterGUID, 10)
}

func (s *SessionOwnershipService) evictionStreamKey(gatewayID string) string {
	return s.realmHashTag() + ":evictions:" + gatewayID
}

func (s *SessionOwnershipService) gatewayLivenessKey(gatewayID string) string {
	return s.realmHashTag() + ":gateway:" + gatewayID
}

func (s *SessionOwnershipService) evictionAckKey(token string, attempt int) string {
	return s.realmHashTag() + ":ack:" + token + ":" + strconv.Itoa(attempt) + ":" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

func scriptInt64(values []any, index int) (int64, error) {
	if index >= len(values) {
		return 0, errors.New("invalid session ownership script response")
	}
	switch value := values[index].(type) {
	case int64:
		return value, nil
	case string:
		return strconv.ParseInt(value, 10, 64)
	default:
		return 0, fmt.Errorf("invalid session ownership script value %T", value)
	}
}

func stringValue(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return ""
	}
}
