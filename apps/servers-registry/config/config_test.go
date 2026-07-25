package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/stretchr/testify/require"
)

func TestLayeringConfigFromEnvironment(t *testing.T) {
	t.Setenv("LAYER_MAPS", "1:2;531:3")

	var layering LayeringConfig
	require.NoError(t, cleanenv.ReadEnv(&layering))
	require.Equal(t, map[uint32]uint32{1: 2, 531: 3}, layering.Maps)
}

func TestLayeringConfigFromEmptyEnvironment(t *testing.T) {
	t.Setenv("LAYER_MAPS", "")

	var layering LayeringConfig
	require.NoError(t, cleanenv.ReadEnv(&layering))
	require.Empty(t, layering.Maps)
}

func TestLayeringConfigFromYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte("maps:\n  1: 2\n  531: 3\n"), 0o600))

	var layering LayeringConfig
	require.NoError(t, cleanenv.ReadConfig(path, &layering))
	require.Equal(t, map[uint32]uint32{1: 2, 531: 3}, layering.Maps)
}
