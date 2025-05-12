package defs

import (
	"fmt"
	"github.com/go-softwarelab/common/pkg/to"
	"time"
)

const (
	// DefaultBSVExchangeUpdateInterval is a duration after which the BSV Exchange Rate should be updated
	DefaultBSVExchangeUpdateInterval = 15 * time.Minute

	// DefaultFiatExchangeUpdateInterval is a duration after which the Fiat Exchange Rate should be updated
	DefaultFiatExchangeUpdateInterval = 24 * time.Hour
)

// WalletServices is a struct that has options for wallet services
type WalletServices struct {
	Chain                           BSVNetwork        `mapstructure:"chain"`
	TaalAPIKey                      string            `mapstructure:"taal_api_key"`
	BitailsAPIKey                   *string           `mapstructure:"bitails_api_key"`
	FiatExchangeRates               FiatExchangeRates `mapstructure:"fiat_exchange_rates"`
	FiatUpdateInterval              *time.Duration    `mapstructure:"fiat_update_interval"`
	DisableMapiCallback             bool              `mapstructure:"disable_mapi_callback"`
	ExchangeratesApiKey             string            `mapstructure:"exchangerates_api_key"`
	ChaintracksFiatExchangeRatesUrl string            `mapstructure:"chaintracks_fiat_exchange_rates_url"`
	Chaintracks                     any               `mapstructure:"chaintracks"` // TODO: create *ChaintracksServiceClient
	ArcURL                          string            `mapstructure:"arc_url"`
	ArcConfig                       any               `mapstructure:"arc"` // TODO: create *ArcConfig

	WhatsOnChain WhatsOnChain `mapstructure:"whats_on_chain"`
}

// Validate checks the validity of the WalletServices struct
func (ws *WalletServices) Validate() error {
	var err error
	if ws.Chain, err = ParseBSVNetworkStr(string(ws.Chain)); err != nil {
		return fmt.Errorf("invalid chain: %w", err)
	}

	if err = ws.FiatExchangeRates.Validate(); err != nil {
		return fmt.Errorf("invalid fiat exchange rates: %w", err)
	}

	if err = ws.WhatsOnChain.BSVExchangeRate.Validate(); err != nil {
		return fmt.Errorf("invalid BSV exchange rate: %w", err)
	}

	// TODO: Double check if ws.WhatsOnChain api key is required

	return nil
}

// WhatsOnChain is a struct that configures WhatsOnChain service
type WhatsOnChain struct {
	APIKey            string          `mapstructure:"api_key"`
	BSVExchangeRate   BSVExchangeRate `mapstructure:"bsv_exchange_rate"`
	BSVUpdateInterval *time.Duration  `mapstructure:"bsv_update_interval"`
}

// DefaultServicesConfig returns a default configuration for wallet services
func DefaultServicesConfig(chain BSVNetwork) WalletServices {
	taalApiKey, arcUrl, babbagePort := networkSpecific(chain)

	ratesTimestamp := time.Date(2023, time.December, 13, 0, 0, 0, 0, time.UTC)

	return WalletServices{
		Chain:      chain,
		TaalAPIKey: taalApiKey,
		WhatsOnChain: WhatsOnChain{
			BSVUpdateInterval: to.Ptr(DefaultBSVExchangeUpdateInterval),
			BSVExchangeRate: BSVExchangeRate{
				Timestamp: ratesTimestamp,
				Base:      USD,
				Rate:      47.52,
			},
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
		ChaintracksFiatExchangeRatesUrl: fmt.Sprintf("https://npm-registry.babbage.systems:%d/getFiatExchangeRates", babbagePort),
		Chaintracks:                     nil, // TODO: implement me
		ArcURL:                          arcUrl,
		ArcConfig:                       nil, // TODO: implement me
	}
}

func networkSpecific(chain BSVNetwork) (taalApiKey, arcURL string, babbagePort int) {
	if chain == NetworkMainnet {
		taalApiKey = "mainnet_9596de07e92300c6287e4393594ae39c" //nolint:gosec // it's a well-known key and can be public
		babbagePort = 8084
		arcURL = "https://api.taal.com/arc"
	} else {
		taalApiKey = "testnet_0e6cf72133b43ea2d7861da2a38684e3" //nolint:gosec // it's a well-known key and can be public
		babbagePort = 8083
		arcURL = "https://arc-test.taal.com/arc"
	}
	return
}
