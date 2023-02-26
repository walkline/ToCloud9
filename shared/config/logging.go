package config

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type LoggingFormat string

const (
	// LoggingFormatDevelopment uses console writer with pretty formatting
	LoggingFormatDevelopment LoggingFormat = "dev"

	// LoggingFormatDevelopment uses JSON format
	LoggingFormatJSON LoggingFormat = "json"
)

// Logging config for logging. Uses tags from github.com/Netflix/go-env.
type Logging struct {
	LogLevel zerolog.Level `env:"LOG_LEVEL,default=0"`
	Format   LoggingFormat `env:"LOG_FORMAT,default=dev"`
}

// Logger creates logger based on config
func (l Logging) Logger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	switch l.Format {
	case LoggingFormatDevelopment:
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "15:04:05.000",
		}).Level(l.LogLevel)
	case LoggingFormatJSON:
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	}
	return log.Logger
}
