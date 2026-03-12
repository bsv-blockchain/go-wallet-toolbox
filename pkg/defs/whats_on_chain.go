package defs

import (
	"fmt"
	"time"
)

// WhatsOnChain is a struct that configures WhatsOnChain service
type WhatsOnChain struct {
	Enabled                    bool            `mapstructure:"enabled"`
	APIKey                     string          `mapstructure:"api_key"`
	BSVExchangeRate            BSVExchangeRate `mapstructure:"bsv_exchange_rate"`
	BSVUpdateInterval          *time.Duration  `mapstructure:"bsv_update_interval"`
	RootForHeightRetryInterval time.Duration   `mapstructure:"root_for_height_retry_interval"`
	RootForHeightRetries       int             `mapstructure:"root_for_height_retries"`
}

// Validate checks if the WhatsOnChain configuration is valid
func (woc *WhatsOnChain) Validate() error {
	if !woc.Enabled {
		return nil
	}

	if err := woc.BSVExchangeRate.Validate(); err != nil {
		return fmt.Errorf("invalid BSV exchange rate: %w", err)
	}

	return nil
}
