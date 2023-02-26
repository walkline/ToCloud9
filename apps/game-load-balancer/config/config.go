package config

import (
	"strconv"

	"github.com/Netflix/go-env"

	game_load_balancer "github.com/walkline/ToCloud9/apps/game-load-balancer"
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging

	// Port is port that would be used to listen the game client
	Port string `env:"PORT,default=8085"`

	// RealmID is id of realm that load balancer works with
	RealmID int `env:"REALM_ID,default=1"`

	// AuthDBConnection is connection string to the auth database
	AuthDBConnection string `env:"AUTH_DB_CONNECTION,default=trinity:trinity@tcp(127.0.0.1:3306)/auth"`

	// HealthCheckPort is port that would be used to listen for health checks
	HealthCheckPort string `env:"HEALTH_CHECK_PORT,default=8900"`

	// PreferredHostname is referred host name that will be used to connect from game client
	PreferredHostname string `env:"PREFERRED_HOSTNAME,default=localhost"`

	// CharServiceAddress is address of characters service
	CharServiceAddress string `env:"CHAR_SERVICE_ADDRESS,default=localhost:8991"`

	// ServersRegistryServiceAddress is address of servers registry service
	ServersRegistryServiceAddress string `env:"SERVERS_REGISTRY_SERVICE_ADDRESS,default=localhost:8999"`

	// ChatServiceAddress is address of chat service
	ChatServiceAddress string `env:"CHAT_SERVICE_ADDRESS,default=localhost:8992"`

	// GuildsServiceAddress is address of guilds service
	GuildsServiceAddress string `env:"GUILDS_SERVICE_ADDRESS,default=localhost:8995"`

	// NatsURL is nats connection url
	NatsURL string `env:"NATS_URL,default=nats://nats:4222"`
}

func (c Config) PortInt() (p int) {
	p, _ = strconv.Atoi(c.Port)
	return
}

func (c Config) HealthCheckPortInt() (p int) {
	p, _ = strconv.Atoi(c.HealthCheckPort)
	return
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var c Config
	_, err := env.UnmarshalFromEnviron(&c)
	if err != nil {
		return nil, err
	}

	game_load_balancer.RealmID = uint32(c.RealmID)

	return &c, nil
}
