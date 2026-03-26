package config_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
)

func TestToYAMLFile(t *testing.T) {
	// given:
	tmpDir := t.TempDir()
	configFilePath := fmt.Sprintf("%s/exported_config.yaml", tmpDir)

	// and:
	cfg := Defaults()

	// when:
	err := config.ToYAMLFile(cfg, configFilePath)

	// then:
	require.NoError(t, err)

	yamlFile, err := os.ReadFile(configFilePath) //nolint:gosec // configFilePath is a test-controlled temp file path
	require.NoError(t, err)

	require.Contains(t, string(yamlFile), "a: default_hello")
	require.Contains(t, string(yamlFile), "b_with_long_name: 1")
	require.Contains(t, string(yamlFile), "d: default_world")
}
