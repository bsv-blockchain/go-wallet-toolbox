package infra_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
)

func TestCaseInsensitiveEnums(t *testing.T) {
	// given:
	setRequiredEnvs(t)
	t.Setenv("TEST_DB_ENGINE", "SQLite")
	t.Setenv("TEST_BSV_NETWORK", "MAIN")
	t.Setenv("TEST_LOGGING_LEVEL", "DeBug")
	t.Setenv("TEST_LOGGING_HANDLER", "Text")
	t.Setenv("TEST_WALLET_SERVICES_WHATS_ON_CHAIN_BSV_EXCHANGE_RATE_BASE", "euR")

	// when:
	infraSrv, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

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
	_, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

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
			setRequiredEnvs(t)
			t.Setenv(test.envKey, "wrong")

			// when:
			_, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

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
			setRequiredEnvs(t)
			t.Setenv("TEST_WALLET_SERVICES_ARC_CALLBACK_URL", test.url)

			// when:
			infraSrv, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

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
			setRequiredEnvs(t)
			t.Setenv("TEST_WALLET_SERVICES_ARC_CALLBACK_URL", test.url)

			// when:
			_, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

			// then:
			require.Error(t, err)
		})
	}
}

func TestInvalidCurrencyForFiatExchangeRates(t *testing.T) {
	// given:
	setRequiredEnvs(t)
	t.Setenv("TEST_WALLET_SERVICES_FIAT_EXCHANGE_RATES_BASE", "PLN")

	// when:
	_, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

	// then:
	require.Error(t, err)
}

func TestValidTimestamp(t *testing.T) {
	// given:
	setRequiredEnvs(t)
	t.Setenv("TEST_WALLET_SERVICES_WHATS_ON_CHAIN_BSV_EXCHANGE_RATE_TIMESTAMP", "2023-12-13T00:00:00Z")

	// when:
	infraSrv, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

	// then:
	require.NoError(t, err)

	// and:
	expected := time.Date(2023, time.December, 13, 0, 0, 0, 0, time.UTC)
	require.Equal(t, expected, infraSrv.Config.Services.WhatsOnChain.BSVExchangeRate.Timestamp)
}

func TestValidMonitor(t *testing.T) {
	// given:
	setRequiredEnvs(t)
	t.Setenv("TEST_MONITOR_ENABLED", "false")

	// when:
	infraSrv, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

	// then:
	require.NoError(t, err)

	// and:

	require.False(t, infraSrv.Config.Monitor.Enabled)
}

func TestMonitorTaskCustomInterval(t *testing.T) {
	// given:
	setRequiredEnvs(t)
	t.Setenv("TEST_MONITOR_TASKS_CHECK_FOR_PROOFS_INTERVAL_SECONDS", "123")
	// when:
	infraSrv, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

	// then:
	require.NoError(t, err)
	require.Equal(t, 123*time.Second, infraSrv.Config.Monitor.Tasks.CheckForProofs.Interval())
}

func TestMonitorTaskZeroInterval(t *testing.T) {
	// given:
	setRequiredEnvs(t)
	t.Setenv("TEST_MONITOR_TASKS_CHECK_FOR_PROOFS_INTERVAL_SECONDS", "0")
	// when:
	_, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

	// then:
	require.Error(t, err)
}

func TestValidHTTPPort(t *testing.T) {
	tests := map[string]struct{ port string }{
		"zero (ephemeral) port": {port: "0"},
		"lowest valid port":     {port: "1"},
		"highest valid port":    {port: "65535"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			setRequiredEnvs(t)
			t.Setenv("TEST_HTTP_PORT", test.port)

			// when:
			_, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

			// then:
			require.NoError(t, err)
		})
	}
}

func TestInvalidHTTPPort(t *testing.T) {
	// given:
	setRequiredEnvs(t)
	t.Setenv("TEST_HTTP_PORT", "65536")

	// when:
	_, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

	// then:
	require.Error(t, err)
}

func TestValidRequestPrice(t *testing.T) {
	tests := map[string]struct{ price string }{
		"free (zero)":    {price: "0"},
		"small positive": {price: "100"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			setRequiredEnvs(t)
			t.Setenv("TEST_HTTP_REQUEST_PRICE", test.price)

			// when:
			_, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

			// then:
			require.NoError(t, err)
		})
	}
}

func TestInvalidRequestPrice(t *testing.T) {
	// given:
	setRequiredEnvs(t)
	// one above the expected MaxSatoshis (21e14)
	t.Setenv("TEST_HTTP_REQUEST_PRICE", "2100000000000001")

	// when:
	_, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

	// then:
	require.Error(t, err)
}

func TestChangeBasketEnvVars(t *testing.T) {
	// given:
	setRequiredEnvs(t)
	t.Setenv("TEST_CHANGE_BASKET_NUMBER_OF_DESIRED_UTXOS", "10000")
	t.Setenv("TEST_CHANGE_BASKET_MINIMUM_DESIRED_UTXO_VALUE", "2000")
	t.Setenv("TEST_CHANGE_BASKET_MAX_CHANGE_OUTPUTS_PER_TX", "50")

	// when:
	infraSrv, err := infra.NewServer(t.Context(), infra.WithEnvPrefix("TEST"))

	// then:
	require.NoError(t, err)
	require.Equal(t, int64(10000), infraSrv.Config.ChangeBasket.NumberOfDesiredUTXOs)
	require.Equal(t, uint64(2000), infraSrv.Config.ChangeBasket.MinimumDesiredUTXOValue)
	require.Equal(t, uint64(50), infraSrv.Config.ChangeBasket.MaxChangeOutputsPerTx)
}

// setRequiredEnvs sets necessary environment variables for test configuration.
// It ensures TEST_SERVER_PRIVATE_KEY is set with a valid private key value for proper test initialization.
func setRequiredEnvs(t *testing.T) {
	t.Setenv("TEST_SERVER_PRIVATE_KEY", fixtures.StorageServerPrivKey)
}
