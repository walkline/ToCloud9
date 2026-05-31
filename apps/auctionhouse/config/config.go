package config

import (
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// Port is port that would be used for grpc server
	Port string `yaml:"port" env:"PORT" env-default:"8993"`

	// CharDBConnection is connection string to the characters database
	CharDBConnection map[uint32]string `yaml:"charactersDB" env:"CHAR_DB_CONNECTION" env-separator:";" env-default:"1:trinity:trinity@tcp(127.0.0.1:3306)/characters"`

	// WorldDBConnection is connection string to the world database (for item templates)
	WorldDBConnection string `yaml:"worldDB" env:"WORLD_DB_CONNECTION" env-default:"trinity:trinity@tcp(127.0.0.1:3306)/acore_world"`

	// NatsURL is nats connection url
	NatsURL string `yaml:"natsUrl" env:"NATS_URL" env-default:"nats://nats:4222"`

	// MailServiceAddress is address of mail service
	MailServiceAddress string `yaml:"mailServiceAddress" env:"MAIL_SERVICE_ADDRESS" env-default:"localhost:8997"`

	// ExpiredAuctionsCheckSecsDelay delay between auction expiration checks.
	ExpiredAuctionsCheckSecsDelay int64 `yaml:"expiredAuctionsCheckSecsDelay" env:"EXPIRED_AUCTIONS_CHECK_SECS_DELAY" env-default:"60"`
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"auctionhouse"`
	}

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	return &c.Root, nil
}