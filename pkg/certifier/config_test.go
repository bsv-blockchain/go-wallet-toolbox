package certifier

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidateRejectsInvalidServerPort(t *testing.T) {
	tests := map[string]string{
		"zero":      "0",
		"too large": "65536",
		"not a int": "not-a-port",
	}
	for name, port := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := ConfigDefaults()
			cfg.Storage.URL = "http://localhost:8100"
			cfg.CertifierWallet.PrivateKey = "set"
			cfg.Server.Port = port

			err := cfg.Validate()

			require.ErrorContains(t, err, "invalid server.port")
		})
	}
}

func TestConfigValidateRejectsNonPositiveRequestBodyLimit(t *testing.T) {
	cfg := ConfigDefaults()
	cfg.Storage.URL = "http://localhost:8100"
	cfg.CertifierWallet.PrivateKey = "set"
	cfg.Server.MaxRequestBodyBytes = 0

	err := cfg.Validate()

	require.ErrorContains(t, err, "server.max_request_body_bytes must be greater than 0")
}
