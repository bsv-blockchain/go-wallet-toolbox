package defs

// WOCPollIngestorConfig holds configuration for polling data from WhatsOnChain, including chain network and API key.
type WOCPollIngestorConfig struct {
	Chain  BSVNetwork `mapstructure:"-"`
	APIKey string     `mapstructure:"api_key"`
}

// Validate checks if the WOCPollIngestorConfig has valid configuration and returns an error if validation fails.
func (c *WOCPollIngestorConfig) Validate() error {
	if err := c.Chain.Validate(); err != nil {
		return err
	}
	return nil
}

// DefaultWOCPollIngestorConfig returns a WOCPollIngestorConfig preconfigured for Bitcoin SV mainnet network.
func DefaultWOCPollIngestorConfig() WOCPollIngestorConfig {
	return WOCPollIngestorConfig{
		Chain: NetworkMainnet,
	}
}
