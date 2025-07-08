package defs

import (
	"fmt"
	"time"
)

// WhatsOnChain is a struct that configures WhatsOnChain service
type WhatsOnChain struct {
	APIKey                     string          `mapstructure:"api_key"`
	BSVExchangeRate            BSVExchangeRate `mapstructure:"bsv_exchange_rate"`
	BroadcastDelay             time.Duration   `mapstructure:"broadcast_delay"`
	BSVUpdateInterval          *time.Duration  `mapstructure:"bsv_update_interval"`
	RootForHeightRetryInterval time.Duration   `mapstructure:"root_for_height_validation_timeout"`
	RootForHeightRetries       int             `mapstructure:"root_for_height_validation_retries"`
}

// Validate checks if the WhatsOnChain configuration is valid
func (woc *WhatsOnChain) Validate() error {
	if err := woc.BSVExchangeRate.Validate(); err != nil {
		return fmt.Errorf("invalid BSV exchange rate: %w", err)
	}

	return nil
}
