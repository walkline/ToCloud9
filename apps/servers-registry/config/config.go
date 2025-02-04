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
