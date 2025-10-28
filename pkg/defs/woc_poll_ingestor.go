package defs

type WOCPollIngestorConfig struct {
	Chain  BSVNetwork `mapstructure:"-"`
	APIKey string     `mapstructure:"api_key"`
}

func (c *WOCPollIngestorConfig) Validate() error {
	if err := c.Chain.Validate(); err != nil {
		return err
	}
	return nil
}

func DefaultWOCPollIngestorConfig() WOCPollIngestorConfig {
	return WOCPollIngestorConfig{
		Chain: NetworkMainnet,
	}
}
