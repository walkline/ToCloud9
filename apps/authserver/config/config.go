package config

import (
	"github.com/Netflix/go-env"

	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging

	// Port is port that would be used to listen the game client
	Port string `env:"PORT,default=3724"`

	// AuthDBConnection is connection string to the auth database
	AuthDBConnection string `env:"AUTH_DB_CONNECTION,default=trinity:trinity@tcp(127.0.0.1:3306)/auth"`

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
