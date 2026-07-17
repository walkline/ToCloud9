package config

import "github.com/walkline/ToCloud9/shared/config"

type Config struct {
	config.Logging         `yaml:"logging"`
	Port                   string `yaml:"port" env:"PORT" env-default:"8996"`
	ServersRegistryAddress string `yaml:"serversRegistryServiceAddress" env:"SERVERS_REGISTRY_SERVICE_ADDRESS" env-default:"localhost:8999"`
	MaxPopulation          uint32 `yaml:"maxPopulation" env:"LAYER_MAX_POPULATION" env-default:"1000"`
	TargetPopulationPct    uint32 `yaml:"targetPopulationPercent" env:"LAYER_TARGET_POPULATION_PERCENT" env-default:"90"`
	OverflowMarginPct      uint32 `yaml:"overflowMarginPercent" env:"LAYER_OVERFLOW_MARGIN_PERCENT" env-default:"10"`
	SwitchCooldownSeconds  uint32 `yaml:"switchCooldownSeconds" env:"LAYER_SWITCH_COOLDOWN_SECONDS" env-default:"60"`
	MaxSwitchesPerHour     uint32 `yaml:"maxSwitchesPerHour" env:"LAYER_MAX_SWITCHES_PER_HOUR" env-default:"6"`
}

func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"layer-coordinator"`
	}
	if err := config.LoadConfig(&c); err != nil {
		return nil, err
	}
	return &c.Root, nil
}
