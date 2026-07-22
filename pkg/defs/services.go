package defs

import (
	"fmt"
	"time"

	"github.com/go-softwarelab/common/pkg/to"
)

const (
	// DefaultBSVExchangeUpdateInterval is a duration after which the BSV Exchange Rate should be updated
	DefaultBSVExchangeUpdateInterval = 15 * time.Minute

	// DefaultFiatExchangeUpdateInterval is a duration after which the Fiat Exchange Rate should be updated
	DefaultFiatExchangeUpdateInterval = 24 * time.Hour

	// DefaultRootForHeightRetryInterval is the timeout for fetching the root for height validation
	DefaultRootForHeightRetryInterval = 1 * time.Second

	// DefaultRootForHeightRetries is the number of retries for fetching the root for height validation
	DefaultRootForHeightRetries = 3

	// ArcURL is the URL for the ARC service
	ArcURL = "https://arc.taal.com"

	// ArcToken is the token for the ARC service - it's a well-known key and can be public
	ArcToken = "mainnet_9596de07e92300c6287e4393594ae39c" //nolint:gosec // well-known public API key

	// ArcTestURL is the URL for the ARC service on testnet
	ArcTestURL = "https://arc-test.taal.com"

	// ArcTestToken is the token for the ARC service on testnet - it's a well-known key and can be public
	ArcTestToken = "testnet_0e6cf72133b43ea2d7861da2a38684e3" //nolint:gosec // well-known public API key

	// BHSTestURL is the URL for the BHS service
	BHSTestURL = "http://localhost:8080"

	// BHSApiKey is the token for the BHS service
	BHSApiKey = ""

	// ChaintracksTestURL is the URL for the ChaintracksClient service
	ChaintracksTestURL = "http://localhost:3011"

	// DefaultGetBeefMaxDepth is the maximum depth for GetBEEF requests
	DefaultGetBeefMaxDepth = 100
)

// Service names
const (
	WhatsOnChainServiceName = "WhatsOnChain"
	BitailsServiceName      = "Bitails"
	ArcServiceName          = "ARC"
	BHSServiceName          = "BHS"
	ChaintracksServiceName  = "Chaintracks"
)

// WalletServices is a struct that has options for wallet services
type WalletServices struct {
	Chain               BSVNetwork        `mapstructure:"-"`
	FiatExchangeRates   FiatExchangeRates `mapstructure:"fiat_exchange_rates"`
	FiatUpdateInterval  *time.Duration    `mapstructure:"fiat_update_interval"`
	ExchangeratesAPIKey string            `mapstructure:"exchangerates_api_key"`
	GetBeefMaxDepth     uint              `mapstructure:"get_beef_max_depth"`

	ArcConfig            ARC               `mapstructure:"arc"`
	Arcade               Arcade            `mapstructure:"arcade"`
	ArcGorillaPoolConfig ARC               `mapstructure:"arc_gorillapool"`
	WhatsOnChain         WhatsOnChain      `mapstructure:"whats_on_chain"`
	Bitails              Bitails           `mapstructure:"bitails"`
	BHS                  BHS               `mapstructure:"bhs"`
	ChaintracksClient    ChaintracksClient `mapstructure:"chaintracks"`
}

// Validate checks the validity of the WalletServices struct
func (ws *WalletServices) Validate() error {
	var err error

	if ws.Chain == "" {
		return fmt.Errorf("chain is required")
	}

	// tstn endpoints are private and supplied at runtime via environment variables.
	// Fail fast with an actionable message when they are missing, instead of surfacing
	// a generic "arcade url is empty" later on.
	if ws.Chain == NetworkTSTN {
		if ws.Arcade.URL == "" {
			return fmt.Errorf("tstn network requires %s to be set in the environment", EnvTstnArcadeURL)
		}
		if ws.ChaintracksClient.Enabled && ws.ChaintracksClient.RemoteURL == "" {
			return fmt.Errorf("tstn network requires %s or %s to be set in the environment", EnvTstnChaintracksURL, EnvTstnArcadeURL)
		}
	}

	if err = ws.FiatExchangeRates.Validate(); err != nil {
		return fmt.Errorf("invalid fiat exchange rates: %w", err)
	}

	if err = ws.WhatsOnChain.Validate(); err != nil {
		return fmt.Errorf("invalid BSV exchange rate: %w", err)
	}

	if err = ws.ArcConfig.Validate(); err != nil {
		return fmt.Errorf("invalid ARC config: %w", err)
	}

	if err = ws.Arcade.Validate(); err != nil {
		return fmt.Errorf("invalid Arcade config: %w", err)
	}

	if err = ws.ArcGorillaPoolConfig.Validate(); err != nil {
		return fmt.Errorf("invalid GorillaPool ARC config: %w", err)
	}

	if err = ws.Bitails.Validate(); err != nil {
		return fmt.Errorf("invalid Bitails config: %w", err)
	}

	if err = ws.ChaintracksClient.Validate(); err != nil {
		return fmt.Errorf("invalid Chaintracks config: %w", err)
	}

	return nil
}

// DefaultServicesConfig returns a default configuration for wallet services
func DefaultServicesConfig(chain BSVNetwork) WalletServices {
	ratesTimestamp := time.Date(2023, time.December, 13, 0, 0, 0, 0, time.UTC)

	ep := endpointsForChain(chain)

	cfg := WalletServices{ //nolint:gosec // G101 - not hardcoded credentials, default config values
		Chain: chain,
		ArcConfig: ARC{
			Enabled: true,
			URL:     ep.arcURL,
			Token:   ep.arcToken,
		},
		Arcade: Arcade{
			Enabled: ep.arcadeEnabled,
			// on networks without an Arcade default the URL is left empty on purpose:
			// flipping Enabled without a URL must not silently hit mainnet - Validate()
			// then forces an explicit URL.
			URL:               ep.arcadeURL,
			EventsURL:         ep.arcadeURL,
			FullStatusUpdates: true,
			CircuitBreaker: ArcadeCircuitBreaker{
				FailureThreshold:           3,
				HealthProbeIntervalSeconds: 30,
			},
		},
		ArcGorillaPoolConfig: ARC{
			Enabled: ep.gorillaEnabled,
			URL:     ep.gorillaURL,
		},
		BHS: BHS{
			Enabled: false,
			URL:     BHSTestURL,
			APIKey:  BHSApiKey,
		},
		WhatsOnChain: WhatsOnChain{
			Enabled:           ep.wocEnabled,
			BSVUpdateInterval: to.Ptr(DefaultBSVExchangeUpdateInterval),
			BSVExchangeRate: BSVExchangeRate{
				Timestamp: ratesTimestamp,
				Base:      USD,
				Rate:      47.52,
			},
			RootForHeightRetryInterval: DefaultRootForHeightRetryInterval,
			RootForHeightRetries:       DefaultRootForHeightRetries,
			RequestsPerSecond:          DefaultWhatsOnChainRequestsPerSecond,
		},
		Bitails: Bitails{
			Enabled:                    false, // NOTE: Bitails is disabled by default
			ScriptHashHistoryPageLimit: defaultScriptHashHistoryPageLimit,
		},
		ChaintracksClient: ChaintracksClient{
			Enabled:   ep.chaintracksEnabled,
			Mode:      ChaintracksClientModeRemote,
			RemoteURL: ep.chaintracksURL,
		},
		FiatExchangeRates: FiatExchangeRates{
			Timestamp: ratesTimestamp,
			Base:      USD,
			Rates: map[Currency]float64{
				USD: 1,
				GBP: 0.8,
				EUR: 0.93,
			},
		},
		FiatUpdateInterval:  to.Ptr(DefaultFiatExchangeUpdateInterval),
		ExchangeratesAPIKey: "bd539d2ff492bcb5619d5f27726a766f",
		GetBeefMaxDepth:     DefaultGetBeefMaxDepth,
	}

	return cfg
}
