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
	ArcToken = "mainnet_9596de07e92300c6287e4393594ae39c" //nolint:gosec

	// ArcTestURL is the URL for the ARC service on testnet
	ArcTestURL = "https://arc-test.taal.com"

	// ArcTestToken is the token for the ARC service on testnet - it's a well-known key and can be public
	ArcTestToken = "testnet_0e6cf72133b43ea2d7861da2a38684e3" //nolint:gosec

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

	ArcConfig         ARC               `mapstructure:"arc"`
	WhatsOnChain      WhatsOnChain      `mapstructure:"whats_on_chain"`
	Bitails           Bitails           `mapstructure:"bitails"`
	BHS               BHS               `mapstructure:"bhs"`
	ChaintracksClient ChaintracksClient `mapstructure:"chaintracks"`
}

// Validate checks the validity of the WalletServices struct
func (ws *WalletServices) Validate() error {
	var err error

	if ws.Chain == "" {
		return fmt.Errorf("chain is required")
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

	cfg := WalletServices{
		Chain: chain,
		ArcConfig: ARC{
			Enabled: true,
			URL:     to.IfThen(chain == NetworkMainnet, ArcURL).ElseThen(ArcTestURL),
			Token:   to.IfThen(chain == NetworkMainnet, ArcToken).ElseThen(ArcTestToken),
		},
		BHS: BHS{
			Enabled: false,
			URL:     BHSTestURL,
			APIKey:  BHSApiKey,
		},
		WhatsOnChain: WhatsOnChain{
			Enabled:           true,
			BSVUpdateInterval: to.Ptr(DefaultBSVExchangeUpdateInterval),
			BSVExchangeRate: BSVExchangeRate{
				Timestamp: ratesTimestamp,
				Base:      USD,
				Rate:      47.52,
			},
			RootForHeightRetryInterval: DefaultRootForHeightRetryInterval,
			RootForHeightRetries:       DefaultRootForHeightRetries,
		},
		Bitails: Bitails{
			Enabled:                    false, // NOTE: Bitails is disabled by default
			ScriptHashHistoryPageLimit: defaultScriptHashHistoryPageLimit,
		},
		ChaintracksClient: ChaintracksClient{
			Enabled:   false,
			Mode:      "remote",
			RemoteURL: ChaintracksTestURL,
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
