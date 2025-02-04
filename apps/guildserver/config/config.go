package config

import (
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// Port is port that would be used for grpc server
	Port string `yaml:"port" env:"PORT" env-default:"8995"`

	// NatsURL is nats connection url
	NatsURL string `yaml:"natsUrl" env:"NATS_URL" env-default:"nats://nats:4222"`

	// CharDBConnection is connection string to the characters database
	CharDBConnection map[uint32]string `yaml:"charactersDB" env:"CHAR_DB_CONNECTION" env-separator:";" env-default:"1:trinity:trinity@tcp(127.0.0.1:3306)/characters"`
}

// LoadConfig loads config from file or/and env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"guild"`
	}

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	return &c.Root, nil
}
