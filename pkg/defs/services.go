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

	// DefaultGetBeefMaxDepth is the maximum depth for GetBEEF requests
	DefaultGetBeefMaxDepth = 100
)

// WalletServices is a struct that has options for wallet services
type WalletServices struct {
	Chain                           BSVNetwork        `mapstructure:"-"`
	FiatExchangeRates               FiatExchangeRates `mapstructure:"fiat_exchange_rates"`
	FiatUpdateInterval              *time.Duration    `mapstructure:"fiat_update_interval"`
	DisableMapiCallback             bool              `mapstructure:"disable_mapi_callback"`
	ExchangeratesApiKey             string            `mapstructure:"exchangerates_api_key"`
	ChaintracksFiatExchangeRatesUrl string            `mapstructure:"chaintracks_fiat_exchange_rates_url"`
	Chaintracks                     any               `mapstructure:"chaintracks"` // TODO: create *ChaintracksServiceClient
	GetBeefMaxDepth                 uint              `mapstructure:"get_beef_max_depth"`

	ArcConfig    ARC          `mapstructure:"arc"`
	WhatsOnChain WhatsOnChain `mapstructure:"whats_on_chain"`
	Bitails      Bitails      `mapstructure:"bitails"`
	BHS          BHS          `mapstructure:"bhs"`
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

	return nil
}

// DefaultServicesConfig returns a default configuration for wallet services
func DefaultServicesConfig(chain BSVNetwork) WalletServices {
	ratesTimestamp := time.Date(2023, time.December, 13, 0, 0, 0, 0, time.UTC)

	cfg := WalletServices{
		Chain: chain,
		BHS: BHS{
			URL:    BHSTestURL,
			APIKey: BHSApiKey,
		},
		WhatsOnChain: WhatsOnChain{
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
			ScriptHashHistoryPageLimit: defaultScriptHashHistoryPageLimit,
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
		FiatUpdateInterval:              to.Ptr(DefaultFiatExchangeUpdateInterval),
		DisableMapiCallback:             true, // rely on WalletMonitor by default
		ExchangeratesApiKey:             "bd539d2ff492bcb5619d5f27726a766f",
		ChaintracksFiatExchangeRatesUrl: "",  // TODO: implement me
		Chaintracks:                     nil, // TODO: implement me
		GetBeefMaxDepth:                 DefaultGetBeefMaxDepth,
	}

	switch chain {
	case NetworkMainnet:
		cfg.ArcConfig.URL = ArcURL
		cfg.ArcConfig.Token = ArcToken
	case NetworkTestnet:
		cfg.ArcConfig.URL = ArcTestURL
		cfg.ArcConfig.Token = ArcTestToken
	default:
		panic("Unsupported chain type: " + string(chain))
	}

	return cfg
}
