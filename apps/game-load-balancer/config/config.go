package config

import (
	"strconv"

	game_load_balancer "github.com/walkline/ToCloud9/apps/game-load-balancer"
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// Port is port that would be used to listen the game client
	Port string `yaml:"port" env:"PORT" env-default:"8085"`

	// RealmID is id of realm that load balancer works with
	RealmID int `yaml:"realmId" env:"REALM_ID" env-default:"1"`

	// AuthDBConnection is connection string to the auth database
	AuthDBConnection string `yaml:"authDB" env:"AUTH_DB_CONNECTION" env-default:"trinity:trinity@tcp(127.0.0.1:3306)/auth"`

	// DBSchemaType is the schema type of database. Supported values: "tc", "ac".
	DBSchemaType string `yaml:"dbSchemaType" env:"DB_SCHEMA_TYPE" env-default:"tc"`

	// HealthCheckPort is port that would be used to listen for health checks
	HealthCheckPort string `yaml:"healthCheckPort" env:"HEALTH_CHECK_PORT" env-default:"8900"`

	// PreferredHostname is referred host name that will be used to connect from game client
	PreferredHostname string `yaml:"preferredHostname" env:"PREFERRED_HOSTNAME" env-default:"localhost"`

	// CharServiceAddress is address of characters service
	CharServiceAddress string `yaml:"charactersServiceAddress" env:"CHAR_SERVICE_ADDRESS" env-default:"localhost:8991"`

	// ServersRegistryServiceAddress is address of servers registry service
	ServersRegistryServiceAddress string `yaml:"serversRegistryServiceAddress" env:"SERVERS_REGISTRY_SERVICE_ADDRESS" env-default:"localhost:8999"`

	// ChatServiceAddress is address of chat service
	ChatServiceAddress string `yaml:"chatServiceAddress" env:"CHAT_SERVICE_ADDRESS" env-default:"localhost:8992"`

	// GuildsServiceAddress is address of guilds service
	GuildsServiceAddress string `yaml:"guildsServiceAddress" env:"GUILDS_SERVICE_ADDRESS" env-default:"localhost:8995"`

	// MailServiceAddress is address of mail service
	MailServiceAddress string `yaml:"mailServiceAddress" env:"MAIL_SERVICE_ADDRESS" env-default:"localhost:8997"`

	// GroupServiceAddress is address of group service
	GroupServiceAddress string `yaml:"groupServiceAddress" env:"GROUP_SERVICE_ADDRESS" env-default:"localhost:8998"`

	// NatsURL is nats connection url
	NatsURL string `yaml:"natsUrl" env:"NATS_URL" env-default:"nats://nats:4222"`
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
	var c struct {
		Root Config `yaml:"game-load-balancer"`
	}

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	game_load_balancer.RealmID = uint32(c.Root.RealmID)

	return &c.Root, nil
}
