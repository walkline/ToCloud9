package groupstatetrace

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	EnvGUIDs    = "TC9_GROUP_STATE_TRACE_GUIDS"
	EnvGUIDFile = "TC9_GROUP_STATE_TRACE_FILE"
	Message     = "TC9_GROUP_STATE_TRACE"
)

type traceConfig struct {
	raw   string
	all   bool
	guids map[uint64]struct{}
}

var traceCache = struct {
	sync.RWMutex
	envRaw   string
	filePath string
	loadedAt time.Time
	cfg      traceConfig
}{}

func Enabled(memberGUIDs ...uint64) bool {
	return currentConfig().matches(memberGUIDs...)
}

func Event(logger *zerolog.Logger, stage string, memberGUIDs ...uint64) *zerolog.Event {
	cfg := currentConfig()
	if !cfg.matches(memberGUIDs...) {
		return nil
	}

	var event *zerolog.Event
	if logger != nil {
		event = logger.Info()
	} else {
		event = log.Info()
	}

	return event.
		Str("tc9Trace", "group-state").
		Str("traceStage", stage).
		Str("traceGUIDs", cfg.raw)
}

func currentConfig() traceConfig {
	envRaw := os.Getenv(EnvGUIDs)
	filePath := os.Getenv(EnvGUIDFile)
	now := time.Now()

	traceCache.RLock()
	if envRaw == traceCache.envRaw && filePath == traceCache.filePath && now.Sub(traceCache.loadedAt) < time.Second {
		cfg := traceCache.cfg
		traceCache.RUnlock()
		return cfg
	}
	traceCache.RUnlock()

	raw := envRaw
	if filePath != "" {
		if data, err := os.ReadFile(filePath); err == nil {
			raw = strings.Trim(envRaw+","+string(data), ", \t\r\n")
		}
	}
	cfg := parseConfig(raw)

	traceCache.Lock()
	traceCache.envRaw = envRaw
	traceCache.filePath = filePath
	traceCache.loadedAt = now
	traceCache.cfg = cfg
	traceCache.Unlock()

	return cfg
}

func parseConfig(raw string) traceConfig {
	cfg := traceConfig{
		raw:   raw,
		guids: map[uint64]struct{}{},
	}

	for _, token := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	}) {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if token == "*" {
			cfg.all = true
			continue
		}

		guid, err := strconv.ParseUint(token, 10, 64)
		if err != nil || guid == 0 {
			continue
		}
		cfg.guids[guid] = struct{}{}
	}

	return cfg
}

func (cfg traceConfig) matches(memberGUIDs ...uint64) bool {
	if cfg.raw == "" {
		return false
	}
	if cfg.all {
		return true
	}
	if len(cfg.guids) == 0 {
		return false
	}

	for _, guid := range memberGUIDs {
		if guid == 0 {
			continue
		}
		if _, ok := cfg.guids[guid]; ok {
			return true
		}
	}

	return false
}
