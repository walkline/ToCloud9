package config

import (
	"github.com/Netflix/go-env"

	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging

	// HealthCheckPort is port that would be used to listen for health checks
	HealthCheckPort string `env:"HEALTH_CHECK_PORT,default=8900"`

	// PreferredHostname is referred host name that will be used to connect from load balancer and for health checks
	PreferredHostname string `env:"PREFERRED_HOSTNAME"`

	// ServersRegistryServiceAddress is address of servers registry service
	ServersRegistryServiceAddress string `env:"SERVERS_REGISTRY_SERVICE_ADDRESS,default=localhost:8999"`
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
