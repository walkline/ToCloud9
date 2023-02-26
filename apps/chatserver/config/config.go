package config

import (
	"github.com/Netflix/go-env"

	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging

	// Port is port that would be used for grpc server
	Port string `env:"PORT,default=8992"`

	// NatsURL is nats connection url
	NatsURL string `env:"NATS_URL,default=nats://nats:4222"`
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
