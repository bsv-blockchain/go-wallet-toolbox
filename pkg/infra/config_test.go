package infra_test

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/infra"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/stretchr/testify/require"
)

func TestCaseInsensitiveEnums(t *testing.T) {
	// given:
	t.Setenv("TEST_SERVER_PRIVATE_KEY", fixtures.StorageServerPrivKey)
	t.Setenv("TEST_DB_ENGINE", "SQLite")
	t.Setenv("TEST_BSV_NETWORK", "MAIN")
	t.Setenv("TEST_LOGGING_LEVEL", "DeBug")
	t.Setenv("TEST_LOGGING_HANDLER", "Text")
	t.Setenv("TEST_WALLET_SERVICES_WHATS_ON_CHAIN_BSV_EXCHANGE_RATE_BASE", "euR")

	// when:
	infraSrv, err := infra.NewServer(infra.WithEnvPrefix("TEST"))

	// then:
	require.NoError(t, err)
	require.Equal(t, defs.DBTypeSQLite, infraSrv.Config.DBConfig.Engine)
	require.Equal(t, defs.NetworkMainnet, infraSrv.Config.BSVNetwork)
	require.Equal(t, defs.LogLevelDebug, infraSrv.Config.Logging.Level)
	require.Equal(t, defs.TextHandler, infraSrv.Config.Logging.Handler)
	require.Equal(t, defs.EUR, infraSrv.Config.Services.WhatsOnChain.BSVExchangeRate.Base)
}

func TestFeeZero(t *testing.T) {
	// given:
	t.Setenv("TEST_FEE_MODEL_VALUE", "0")

	// when:
	_, err := infra.NewServer(infra.WithEnvPrefix("TEST"))

	// then:
	require.Error(t, err)
}

func TestEnums(t *testing.T) {
	tests := map[string]struct {
		envKey string
	}{
		"DB engine": {
			envKey: "TEST_DB_ENGINE",
		},
		"BSV network": {
			envKey: "TEST_BSV_NETWORK",
		},
		"HTTP port": {
			envKey: "TEST_HTTP_PORT",
		},
		"Logging level": {
			envKey: "TEST_LOGGING_LEVEL",
		},
		"Logging handler": {
			envKey: "TEST_LOGGING_HANDLER",
		},
		"Fee model": {
			envKey: "TEST_FEE_MODEL_TYPE",
		},
		"Currency on whats on chain exchange rate": {
			envKey: "TEST_WALLET_SERVICES_WHATS_ON_CHAIN_BSV_EXCHANGE_RATE_BASE",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			t.Setenv("TEST_SERVER_PRIVATE_KEY", fixtures.StorageServerPrivKey)
			t.Setenv(test.envKey, "wrong")

			// when:
			_, err := infra.NewServer(infra.WithEnvPrefix("TEST"))

			// then:
			require.Error(t, err)
		})
	}
}

func TestValidArcCallbacks(t *testing.T) {
	tests := map[string]struct {
		url string
	}{
		"empty url is valid and means - callbacks are disabled": {
			url: "",
		},
		"http": {
			url: "http://example.com",
		},
		"https": {
			url: "https://example.com",
		},
		"subdomain, http": {
			url: "http://subdomain.example.com",
		},
		"subdomain, https": {
			url: "https://subdomain.example.com",
		},
		"port, http": {
			url: "http://example.com:3003",
		},
		"port, https": {
			url: "https://example.com:3003",
		},
		"subdomain, port, http": {
			url: "http://subdomain.example.com:3003",
		},
		"subdomain, port, https": {
			url: "https://subdomain.example.com:3003",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			t.Setenv("TEST_SERVER_PRIVATE_KEY", fixtures.StorageServerPrivKey)
			t.Setenv("TEST_WALLET_SERVICES_ARC_CALLBACK_URL", test.url)

			// when:
			infraSrv, err := infra.NewServer(infra.WithEnvPrefix("TEST"))

			// then:
			require.NoError(t, err)

			// and:
			require.Equal(t, test.url, infraSrv.Config.Services.ArcConfig.CallbackURL)
		})
	}
}

func TestInvalidArcCallbacks(t *testing.T) {
	tests := map[string]struct {
		url string
	}{
		"external url without schema is invalid callback url": {
			url: "example.com",
		},
		"external url with ftp schema is invalid callback url": {
			url: "ftp://example.com",
		},
		"localhost is invalid callback url": {
			url: "https://localhost",
		},
		"localhost IP is invalid callback url": {
			url: "https://127.0.0.1",
		},
		"local network address is invalid callback url": {
			url: "https://10.0.0.1",
		},
		"url with wrong https schema part (no colon) is invalid callback url": {
			url: "https//example.com",
		},
		"url with wrong http schema part (no colon) is invalid callback url": {
			url: "http//example.com",
		},
		"not a valid url": {
			url: "not a valid url",
		},
		"url without http prefix": {
			url: "example.com",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			t.Setenv("TEST_SERVER_PRIVATE_KEY", fixtures.StorageServerPrivKey)
			t.Setenv("TEST_WALLET_SERVICES_ARC_CALLBACK_URL", test.url)

			// when:
			_, err := infra.NewServer(infra.WithEnvPrefix("TEST"))

			// then:
			require.Error(t, err)
		})
	}
}

func TestInvalidCurrencyForFiatExchangeRates(t *testing.T) {
	// given:
	t.Setenv("TEST_SERVER_PRIVATE_KEY", fixtures.StorageServerPrivKey)
	t.Setenv("TEST_WALLET_SERVICES_FIAT_EXCHANGE_RATES_BASE", "PLN")

	// when:
	_, err := infra.NewServer(infra.WithEnvPrefix("TEST"))

	// then:
	require.Error(t, err)
}

