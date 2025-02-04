package config

import (
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// Port is port that would be used for grpc server
	Port string `yaml:"port" env:"PORT" env-default:"8994"`

	// CharDBConnection is connection string to the characters database
	CharDBConnection map[uint32]string `yaml:"charactersDB" env:"CHAR_DB_CONNECTION" env-separator:";" env-default:"1:trinity:trinity@tcp(127.0.0.1:3306)/characters"`

	// WorldDBConnection is connection string to the world database
	WorldDBConnection string `yaml:"worldDB" env:"WORLD_DB_CONNECTION" env-default:"trinity:trinity@tcp(127.0.0.1:3306)/world"`

	// NatsURL is nats connection url
	NatsURL string `yaml:"natsUrl" env:"NATS_URL" env-default:"nats://nats:4222"`

	// ServersRegistryServiceAddress is address of servers registry service
	ServersRegistryServiceAddress string `yaml:"serversRegistryServiceAddress" env:"SERVERS_REGISTRY_SERVICE_ADDRESS" env-default:"localhost:8999"`

	// BattleGroups are unions of realms that can participate in PvP between each other (such as battlegrounds).
	// If you don't want to have cross-realm - simply leave it empty.
	// To create a BattleGroup simply set some id for battle group and specify realms ids, example:
	//   battleGroups:
	//		1: "1,2"
	//		2: "3,4"
	// In this example we created two battle groups with ID 1 and 2.
	// Battle group 1 consist from realms with ID 1 and 2.
	// Battle group 2 consist from realms with ID 3 and 4.
	BattleGroups map[uint32]string `yaml:"battleGroups" env:"BATTLE_GROUPS" env-separator:";" env-default:"1:1"`
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"matchmakingserver"`
	}

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	return &c.Root, nil
}
