package config

import (
	"github.com/Netflix/go-env"

	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging

	// Port is port that would be used for grpc server
	Port string `env:"PORT,default=8995"`

	// NatsURL is nats connection url
	NatsURL string `env:"NATS_URL,default=nats://nats:4222"`

	// CharDBConnection is connection string to the characters database
	CharDBConnection string `env:"CHAR_DB_CONNECTION,default=trinity:trinity@tcp(127.0.0.1:3306)/characters"`
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
