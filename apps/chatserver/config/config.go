package config

import (
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// Port is port that would be used for grpc server
	Port string `yaml:"port" env:"PORT" env-default:"8992"`

	// NatsURL is nats connection url
	NatsURL string `yaml:"natsUrl" env:"NATS_URL" env-default:"nats://nats:4222"`

	// CharDBConnection is connection string to the characters database
	CharDBConnection map[uint32]string `yaml:"charactersDB" env:"CHAR_DB_CONNECTION" env-separator:";" env-default:"1:trinity:trinity@tcp(127.0.0.1:3306)/characters"`

	// CharServiceAddress is address of characters service
	CharServiceAddress string `yaml:"charServiceAddress" env:"CHAR_SERVICE_ADDRESS" env-default:"localhost:8991"`

	// ChannelCleanupInterval is how often to run channel cleanup (e.g., "24h", "6h")
	// Safe to run on all instances - cleanup is idempotent with automatic jitter
	ChannelCleanupInterval string `yaml:"channelCleanupInterval" env:"CHANNEL_CLEANUP_INTERVAL" env-default:"24h"`

	// ChannelInactiveThreshold is how long a channel must be inactive before cleanup (e.g., "720h" = 30 days)
	ChannelInactiveThreshold string `yaml:"channelInactiveThreshold" env:"CHANNEL_INACTIVE_THRESHOLD" env-default:"720h"`
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"chat"`
	}

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	return &c.Root, nil
}
