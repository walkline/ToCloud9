package config

import (
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// Port is port that would be used for grpc server
	Port string `yaml:"port" env:"PORT" env-default:"8997"`

	// CharDBConnection is connection string to the characters database
	CharDBConnection map[uint32]string `yaml:"charactersDB" env:"CHAR_DB_CONNECTION" env-separator:";" env-default:"1:trinity:trinity@tcp(127.0.0.1:3306)/characters"`

	// NatsURL is nats connection url
	NatsURL string `yaml:"natsUrl" env:"NATS_URL" env-default:"nats://nats:4222"`

	// ExpiredMailsCleanupSecsDelay delay between job that cleans up expired mails.
	ExpiredMailsCleanupSecsDelay int64 `yaml:"expiredMailsCleanupSecsDelay" env:"EXPIRED_MAILS_CLEANUP_SECS_DELAY" env-default:"3600"`

	// DefaultMailExpirationTimeSecs is default mail expiration time if client doesnt provide one.
	// By default - 2592000 - 30 days.
	DefaultMailExpirationTimeSecs int64 `yaml:"defaultMailExpirationTimeSecs" env:"DEFAULT_MAIL_EXPIRATION_TIME_SECS" env-default:"2592000"`
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"mail"`
	}

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	return &c.Root, nil
}
