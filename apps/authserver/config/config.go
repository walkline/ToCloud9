package config

import (
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// Port is port that would be used to listen the game client
	Port string `yaml:"port" env:"PORT" env-default:"3724"`

	// DBSchemaType is the schema type of database. Supported values: "tc", "ac", "cm".
	DBSchemaType string `yaml:"dbSchemaType" env:"DB_SCHEMA_TYPE" env-default:"tc"`

	// AuthDBConnection is connection string to the auth database
	AuthDBConnection string `yaml:"authDB" env:"AUTH_DB_CONNECTION" env-default:"trinity:trinity@tcp(127.0.0.1:3306)/auth"`

	// ServersRegistryServiceAddress is address of servers registry service
	ServersRegistryServiceAddress string `yaml:"serversRegistryServiceAddress" env:"SERVERS_REGISTRY_SERVICE_ADDRESS" env-default:"localhost:8999"`
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var C struct {
		Root Config `yaml:"auth"`
	}

	err := config.LoadConfig(&C)
	if err != nil {
		return nil, err
	}

	return &C.Root, nil
}
