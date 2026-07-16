package config

import (
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// Port is port that would be used for grpc server
	Port string `yaml:"port" env:"PORT" env-default:"8999"`

	// RedisConnection is connection string for the redis connection
	RedisConnection string `yaml:"redisUrl" env:"REDIS_URL" env-default:"redis://:@redis:6379/0"`

	// NatsURL is nats connection url
	NatsURL string `yaml:"natsUrl" env:"NATS_URL" env-default:"nats://nats:4222"`

	// RealmsIDs is id of realms that the system supports.
	RealmsID []uint32 `yaml:"realmsID" env:"REALMs_ID" env-default:"1"`

	Layering LayeringConfig `yaml:"layering"`
}

type LayeringConfig struct {
	Enabled               bool                   `yaml:"enabled" env:"LAYERING_ENABLED" env-default:"false"`
	MaxPopulation         uint32                 `yaml:"maxPopulation" env:"LAYER_MAX_POPULATION" env-default:"1000"`
	TargetPopulationPct   uint32                 `yaml:"targetPopulationPercent" env:"LAYER_TARGET_POPULATION_PERCENT" env-default:"90"`
	OverflowMarginPct     uint32                 `yaml:"overflowMarginPercent" env:"LAYER_OVERFLOW_MARGIN_PERCENT" env-default:"10"`
	MinCapacityPct        uint32                 `yaml:"minCapacityPercent" env:"LAYER_MIN_CAPACITY_PERCENT" env-default:"10"`
	MinCapacityDuration   uint32                 `yaml:"minCapacityDurationSeconds" env:"LAYER_MIN_CAPACITY_DURATION_SECONDS" env-default:"300"`
	SwitchCooldownSeconds uint32                 `yaml:"switchCooldownSeconds" env:"LAYER_SWITCH_COOLDOWN_SECONDS" env-default:"60"`
	MaxSwitchesPerHour    uint32                 `yaml:"maxSwitchesPerHour" env:"LAYER_MAX_SWITCHES_PER_HOUR" env-default:"6"`
	MinLayers             uint32                 `yaml:"minLayers" env:"LAYER_MIN_LAYERS" env-default:"1"`
	MaxLayers             uint32                 `yaml:"maxLayers" env:"LAYER_MAX_LAYERS" env-default:"10"`
	ReconcileIntervalSecs uint32                 `yaml:"reconcileIntervalSeconds" env:"LAYER_RECONCILE_INTERVAL_SECONDS" env-default:"5"`
	Scopes                []LayerScopeConfig     `yaml:"scopes"`
	ScopeMapIDs           []uint32               `yaml:"-" env:"LAYER_SCOPE_MAP_IDS"`
	ScopeZoneIDs          []uint32               `yaml:"-" env:"LAYER_SCOPE_ZONE_IDS"`
	ScopeMaxPopulation    uint32                 `yaml:"-" env:"LAYER_SCOPE_MAX_POPULATION" env-default:"0"`
	Provisioner           LayerProvisionerConfig `yaml:"provisioner"`
}

type LayerScopeConfig struct {
	Name          string   `yaml:"name"`
	MapIDs        []uint32 `yaml:"mapIDs"`
	ZoneIDs       []uint32 `yaml:"zoneIDs"`
	MaxPopulation uint32   `yaml:"maxPopulation"`
}

type LayerProvisionerConfig struct {
	Type            string   `yaml:"type" env:"LAYER_PROVISIONER_TYPE" env-default:"none"`
	Namespace       string   `yaml:"namespace" env:"LAYER_KUBERNETES_NAMESPACE" env-default:"default"`
	BaseDeployments []string `yaml:"baseDeployments" env:"LAYER_BASE_DEPLOYMENTS"`
	NamePrefix      string   `yaml:"namePrefix" env:"LAYER_DEPLOYMENT_PREFIX" env-default:"tc9"`
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"servers-registry"`
	}

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	return &c.Root, nil
}
