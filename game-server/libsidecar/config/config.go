package config

import (
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// GRPCPort is port that would be used to listen for GRPC requests
	GRPCPort string `yaml:"grpcPort" env:"GRPC_PORT" env-default:"9501"`

	// HealthCheckPort is port that would be used to listen for health checks
	HealthCheckPort string `yaml:"healthCheckPort" env:"HEALTH_CHECK_PORT" env-default:"8901"`

	// PreferredHostname is referred host name that will be used to connect from load balancer and for health checks
	PreferredHostname string `yaml:"preferredHostname" env:"PREFERRED_HOSTNAME"`

	// ServersRegistryServiceAddress is address of servers registry service
	ServersRegistryServiceAddress string `yaml:"serversRegistryServiceAddress" env:"SERVERS_REGISTRY_SERVICE_ADDRESS" env-default:"localhost:8999"`

	// MatchmakingServiceAddress is address of matchmaking service
	MatchmakingServiceAddress string `yaml:"matchmakingServiceAddress" env:"MATCHMAKING_SERVICE_ADDRESS" env-default:"localhost:8994"`

	// GuidProviderServiceAddress is address of service that provides guids to use
	GuidProviderServiceAddress string `yaml:"guidProviderServiceAddress" env:"GUID_PROVIDER_SERVICE_ADDRESS" env-default:"localhost:8996"`

	// CharacterGuidsBufferSize is the size of the buffer for characters guids
	CharacterGuidsBufferSize int `yaml:"characterGuidsBufferSize" env:"CHARACTER_GUIDS_BUFFER_SIZE" env-default:"50"`

	// CharacterGuidsBufferSize is the size of the buffer for items guids
	ItemGuidsBufferSize int `yaml:"itemGuidsBufferSize" env:"ITEM_GUIDS_BUFFER_SIZE" env-default:"200"`

	// InstanceGuidsBufferSize is the size of the buffer for dungeon/raid instances guids
	InstanceGuidsBufferSize int `yaml:"instanceGuidsBufferSize" env:"INSTANCE_GUIDS_BUFFER_SIZE" env-default:"10"`

	// NatsURL is nats connection url
	NatsURL string `yaml:"natsUrl" env:"NATS_URL" env-default:"nats://localhost:4222"`
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"gameserver"`
	}

	config.EnvVarConfigFilePath = "TC9_CONFIG_FILE"
	config.ConfigPathFlagName = "tc9config"

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	return &c.Root, nil
}
