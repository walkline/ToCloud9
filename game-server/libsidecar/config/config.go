package config

import (
	"github.com/Netflix/go-env"

	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging

	// GRPCPort is port that would be used to listen for GRPC requests
	GRPCPort string `env:"GRPC_PORT,default=9501"`

	// HealthCheckPort is port that would be used to listen for health checks
	HealthCheckPort string `env:"HEALTH_CHECK_PORT,default=8900"`

	// PreferredHostname is referred host name that will be used to connect from load balancer and for health checks
	PreferredHostname string `env:"PREFERRED_HOSTNAME"`

	// ServersRegistryServiceAddress is address of servers registry service
	ServersRegistryServiceAddress string `env:"SERVERS_REGISTRY_SERVICE_ADDRESS,default=localhost:8999"`

	// GuidProviderServiceAddress is address of service that provides guids to use
	GuidProviderServiceAddress string `env:"GUID_PROVIDER_SERVICE_ADDRESS,default=localhost:8996"`

	// CharacterGuidsBufferSize is the size of the buffer for characters guids
	CharacterGuidsBufferSize int `env:"CHARACTER_GUIDS_BUFFER_SIZE,default=50"`

	// CharacterGuidsBufferSize is the size of the buffer for items guids
	ItemGuidsBufferSize int `env:"ITEM_GUIDS_BUFFER_SIZE,default=200"`

	// NatsURL is nats connection url
	NatsURL string `env:"NATS_URL,default=nats://localhost:4222"`
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var c Config
	_, err := env.UnmarshalFromEnviron(&c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}
