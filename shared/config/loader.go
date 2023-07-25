package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ilyakaznacheev/cleanenv"
)

var (
	// EnvVarConfigFilePath environment variable to set config path.
	EnvVarConfigFilePath = "CONFIG_FILE"

	// DefaultConfigName default name of the config file.
	DefaultConfigName = "config.yml"

	// ConfigPathFlagName asd
	ConfigPathFlagName = "c"
)

// LoadConfig loads config from file and/or environment variables.
// You can provide config file with `-c` argument or with `CONFIG_FILE` environment variable.
// If config file path not provided, then looks for the `config.file` in pwd or in executable directory.
func LoadConfig(cfg interface{}) error {
	file, err := parseArgsForConfigFile(cfg)
	if err != nil {
		return err
	}

	if file == "" {
		file = parseEnvForConfigFile()
	}

	if file == "" {
		path, err := os.Getwd()
		if err != nil {
			return err
		}

		defaultPwdConfigPath := filepath.Join(path, DefaultConfigName)
		if hasFileWithPath(defaultPwdConfigPath) {
			file = defaultPwdConfigPath
		}
	}

	if file == "" {
		path, err := os.Executable()
		if err != nil {
			return err
		}

		defaultExecutableConfigPath := filepath.Join(filepath.Dir(path), DefaultConfigName)
		if hasFileWithPath(defaultExecutableConfigPath) {
			file = defaultExecutableConfigPath
		}
	}

	if file != "" {
		// Reads env vars as well.
		return cleanenv.ReadConfig(file, cfg)
	}

	return cleanenv.ReadEnv(cfg)
}

func parseArgsForConfigFile(cfg interface{}) (path string, err error) {
	f := flag.NewFlagSet("ToCloud9 server", flag.ContinueOnError)
	f.StringVar(&path, ConfigPathFlagName, "", "Path to configuration file")

	fu := f.Usage
	f.Usage = func() {
		fu()
		envHelp, _ := cleanenv.GetDescription(cfg, nil)
		_, _ = fmt.Fprintln(f.Output())
		_, _ = fmt.Fprintln(f.Output(), envHelp)
	}

	err = f.Parse(os.Args[1:])
	if err != nil {
		fmt.Println("parsing args failed, err: ", err)
		// Let's not fail the whole thing.
		err = nil
	}
	return
}

func parseEnvForConfigFile() string {
	return os.Getenv(EnvVarConfigFilePath)
}

func hasFileWithPath(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
