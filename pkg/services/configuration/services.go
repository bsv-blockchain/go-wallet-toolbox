package configuration

import (
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

// WalletServices is a struct that has options for wallet services
type WalletServices struct {
	Chain                           defs.BSVNetwork       `mapstructure:"chain"`
	BitailsAPIKey                   *string               `mapstructure:"bitails_api_key"`
	FiatExchangeRates               wdk.FiatExchangeRates `mapstructure:"fiat_exchange_rates"`
	FiatUpdateInterval              *time.Duration        `mapstructure:"fiat_update_interval"`
	DisableMapiCallback             bool                  `mapstructure:"disable_mapi_callback"`
	ExchangeratesApiKey             string                `mapstructure:"exchangerates_api_key"`
	ChaintracksFiatExchangeRatesUrl string                `mapstructure:"chaintracks_fiat_exchange_rates_url"`
	Chaintracks                     any                   `mapstructure:"chaintracks"` // TODO: create *ChaintracksServiceClient

	ArcConfig    ARC          `mapstructure:"arc"`
	WhatsOnChain WhatsOnChain `mapstructure:"woc"`
}
