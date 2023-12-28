package config

import (
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// Port is port that would be used for grpc server
	Port string `yaml:"port" env:"PORT" env-default:"8998"`

	// NatsURL is nats connection url
	NatsURL string `yaml:"natsUrl" env:"NATS_URL" env-default:"nats://nats:4222"`

	// CharDBConnection is connection string to the characters database
	CharDBConnection string `yaml:"charactersDB" env:"CHAR_DB_CONNECTION" env-default:"trinity:trinity@tcp(127.0.0.1:3306)/characters"`

	// CharServiceAddress is address of characters service
	CharServiceAddress string `yaml:"charactersServiceAddress" env:"CHAR_SERVICE_ADDRESS" env-default:"localhost:8991"`
}

// LoadConfig loads config from file or/and env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"group"`
	}

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	return &c.Root, nil
}
