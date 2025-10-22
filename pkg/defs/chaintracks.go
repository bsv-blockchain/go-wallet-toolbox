package defs

import "fmt"

type ChaintracksServiceConfig struct {
	Chain BSVNetwork `mapstructure:"-"`
}

func (c *ChaintracksServiceConfig) Validate() error {
	if err := c.Chain.Validate(); err != nil {
		return fmt.Errorf("invalid chain: %w", err)
	}
	return nil
}

func DefaultChaintracksServiceConfig() ChaintracksServiceConfig {
	return ChaintracksServiceConfig{
		Chain: NetworkMainnet,
	}
}

type ChaintracksServerConfig struct {
	ChaintracksServiceConfig
	Port  uint       `mapstructure:"port"`
}

func (c *ChaintracksServerConfig) Validate() error {
	if err := c.ChaintracksServiceConfig.Validate(); err != nil {
		return fmt.Errorf("invalid chaintracks service config: %w", err)
	}

	const maxPort = 65535
	if c.Port > maxPort {
		return fmt.Errorf("invalid port: %d", c.Port)
	}

	return nil
}

func DefaultChaintracksServerConfig() ChaintracksServerConfig {
	return ChaintracksServerConfig{
		Port: 3011,
		ChaintracksServiceConfig: DefaultChaintracksServiceConfig(),
	}
}
