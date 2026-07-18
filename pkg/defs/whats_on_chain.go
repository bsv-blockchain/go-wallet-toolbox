package defs

import (
	"fmt"
	"time"
)

// DefaultWhatsOnChainRequestsPerSecond is the WhatsOnChain rate limit for requests
// without an API key.
const DefaultWhatsOnChainRequestsPerSecond = 3

// WhatsOnChain is a struct that configures WhatsOnChain service
type WhatsOnChain struct {
	Enabled                    bool            `mapstructure:"enabled"`
	APIKey                     string          `mapstructure:"api_key"`
	BSVExchangeRate            BSVExchangeRate `mapstructure:"bsv_exchange_rate"`
	BSVUpdateInterval          *time.Duration  `mapstructure:"bsv_update_interval"`
	RootForHeightRetryInterval time.Duration   `mapstructure:"root_for_height_retry_interval"`
	RootForHeightRetries       int             `mapstructure:"root_for_height_retries"`

	// RequestsPerSecond throttles all requests to WhatsOnChain on the client side, so
	// the service's rate limit is not exceeded (which would yield 429 responses and turn
	// into broadcast failures). When 0, DefaultWhatsOnChainRequestsPerSecond is used -
	// the limit WhatsOnChain applies to requests without an API key. With an API key the
	// value should be set to the limit of the purchased plan (e.g. 10).
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`
}

// Validate checks if the WhatsOnChain configuration is valid
func (woc *WhatsOnChain) Validate() error {
	if !woc.Enabled {
		return nil
	}

	if err := woc.BSVExchangeRate.Validate(); err != nil {
		return fmt.Errorf("invalid BSV exchange rate: %w", err)
	}

	if woc.RequestsPerSecond < 0 {
		return fmt.Errorf("requests per second cannot be negative")
	}

	return nil
}
