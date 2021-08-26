package config

import (
	"github.com/Netflix/go-env"

	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging

	// Port is port that would be used to serve grpc server
	Port string `env:"PORT,default=8991"`

	// CharDBConnection is connection string to the characters database
	CharDBConnection string `env:"CHAR_DB_CONNECTION,default=trinity:trinity@tcp(127.0.0.1:3306)/characters"`

	// WorldDBConnection is connection string to the world database
	WorldDBConnection string `env:"WORLD_DB_CONNECTION,default=trinity:trinity@tcp(127.0.0.1:3306)/world"`
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
