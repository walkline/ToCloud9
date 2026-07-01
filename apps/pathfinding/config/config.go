package config

import (
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of pathfinding service
type Config struct {
	config.Logging `yaml:"logging"`

	// Port is port that would be used for grpc server
	Port string `yaml:"port" env:"PORT" env-default:"8999"`

	// MmapsDir is path to the directory containing mmap files
	MmapsDir string `yaml:"mmapsDir" env:"MMAPS_DIR" env-default:"mmaps"`

	// MapsDir is path to the directory containing .map terrain files for accurate height (Z)
	MapsDir string `yaml:"mapsDir" env:"MAPS_DIR" env-default:"maps"`
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"pathfinding"`
	}

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	return &c.Root, nil
}