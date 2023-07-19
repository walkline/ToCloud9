package config

import (
	"github.com/Netflix/go-env"

	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging

	// Port is port that would be used for grpc server
	Port string `env:"PORT,default=8995"`

	// CharDBConnection is connection string to the characters database
	CharDBConnection string `env:"CHAR_DB_CONNECTION,default=trinity:trinity@tcp(127.0.0.1:3306)/characters"`

	// NatsURL is nats connection url
	NatsURL string `env:"NATS_URL,default=nats://nats:4222"`

	// ExpiredMailsCleanupSecsDelay delay between job that cleans up expired mails.
	ExpiredMailsCleanupSecsDelay int64 `env:"EXPIRED_MAILS_CLEANUP_SECS_DELAY,default=3600"`

	// DefaultMailExpirationTimeSecs is default mail expiration time if client doesnt provide one.
	// By default - 2592000 - 30 days.
	DefaultMailExpirationTimeSecs int64 `env:"DEFAULT_MAIL_EXPIRATION_TIME_SECS,default=2592000"`
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
