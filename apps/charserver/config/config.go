package config

import (
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// Port is port that would be used to serve grpc server
	Port string `yaml:"port" env:"PORT" env-default:"8991"`

	// CharDBConnection is connection string to the characters database
	CharDBConnection string `yaml:"charactersDB" env:"CHAR_DB_CONNECTION" env-default:"trinity:trinity@tcp(127.0.0.1:3306)/characters"`

	// WorldDBConnection is connection string to the world database
	WorldDBConnection string `yaml:"worldDB" env:"WORLD_DB_CONNECTION" env-default:"trinity:trinity@tcp(127.0.0.1:3306)/world"`

	// NatsURL is nats connection url
	NatsURL string `yaml:"natsUrl" env:"NATS_URL" env-default:"nats://nats:4222"`
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"characters"`
	}

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	return &c.Root, nil
}
