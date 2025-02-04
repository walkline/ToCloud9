package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	// Apps is list of apps to manage
	Apps []AppConfig `yaml:"apps" env:"-" env-default:""`
}

type AppConfig struct {
	// Name is the name of application. Can differ from binary name.
	Name string `yaml:"name"`

	// Alias is used as a reference to this app.
	// If you set alias like "aa", then you can use it in commands, example: "restart aa" will restart this app.
	Alias []string `yaml:"alias"`

	// Binary is path to the binary/executable file.
	Binary string `yaml:"binary"`

	// Args is app argument variables that will be passed on launch.
	Args []string `yaml:"args"`

	// Env is environment variables to pass to the app.
	Env map[string]string `yaml:"env"`

	// WorkingDir is a working dir that will be passed to the app.
	WorkingDir string `yaml:"workingDir"`

	// StartupTimeoutSecs is timeout time for starting the app in seconds.
	StartupTimeoutSecs int `yaml:"startupTimeoutSecs"`

	// PartOfAppStartedLogMsg used to determine if app started.
	// Example: If your app prints log like this when app started: "MySupper app successfully started.",
	// then you can set this variable to "successfully started" or "started".
	PartOfAppStartedLogMsg string `yaml:"partOfAppStartedLogMsg"`
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"perun"`
	}

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	if len(c.Root.Apps) == 0 {
		return nil, errors.New("apps can't be empty")
	}

	for i, app := range c.Root.Apps {
		c.Root.Apps[i].Binary, err = makeAbsolutePath(app.Binary)
		if err != nil {
			return nil, fmt.Errorf("can't find executable '%s'", app.Binary)
		}
	}

	return &c.Root, nil
}

func makeAbsolutePath(filePath string) (string, error) {
	if filepath.IsAbs(filePath) && fileExists(filePath) {
		return filePath, nil // Return as is if it's already absolute and exists
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	absPath := filepath.Join(cwd, filePath)
	if fileExists(absPath) {
		// File exists in current working directory, return absolute path
		return absPath, nil
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	exeDir := filepath.Dir(executablePath)
	absPath = filepath.Join(exeDir, filePath)

	if !fileExists(absPath) {
		return "", fmt.Errorf("file does not exist: %s", absPath)
	}

	return absPath, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
