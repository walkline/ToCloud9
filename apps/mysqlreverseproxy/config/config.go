package config

import (
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// Port is port that would be used to connect to this mysql proxy
	Port string `yaml:"port" env:"PORT" env-default:"3307"`

	// CharDBConnection is connection string to the characters database by realm id.
	// 0 realm id means the default cross realm character database that will be used when can't use realm specific one.
	CharDBConnection map[uint32]string `yaml:"charactersDB" env:"CHAR_DB_CONNECTION" env-separator:";" env-default:"1:trinity:trinity@tcp(127.0.0.1:3306)/characters"`

	// Username is mysql user that will be used to connect to this server.
	Username string `yaml:"username" env:"MYSQL_USER" env-default:"root"`
	// Password is mysql user that will be used to connect to this server.
	Password string `yaml:"password" env:"MYSQL_PASSWORD" env-default:""`
}

// LoadConfig loads config from file or/and env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"mysqlreverseproxy"`
	}

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	return &c.Root, nil
}
