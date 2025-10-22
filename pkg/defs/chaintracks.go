package defs

import "fmt"

// ChaintracksServiceConfig holds configuration for Chaintracks service, including the BSV network selection.
type ChaintracksServiceConfig struct {
	Chain BSVNetwork `mapstructure:"-"`
}

// Validate checks if the Chain field in ChaintracksServiceConfig holds a valid BSV network type.
// Returns an error if the Chain value is not supported.
// Intended to ensure correct service configuration before server initialization or handler registration.
func (c *ChaintracksServiceConfig) Validate() error {
	if err := c.Chain.Validate(); err != nil {
		return fmt.Errorf("invalid chain: %w", err)
	}
	return nil
}

// DefaultChaintracksServiceConfig returns a ChaintracksServiceConfig set to use the mainnet BSV network by default.
func DefaultChaintracksServiceConfig() ChaintracksServiceConfig {
	return ChaintracksServiceConfig{
		Chain: NetworkMainnet,
	}
}

// ChaintracksServerConfig holds the configuration for the Chaintracks HTTP server and its underlying service settings.
type ChaintracksServerConfig struct {
	ChaintracksServiceConfig
	Port          uint   `mapstructure:"port"`
	RoutingPrefix string `mapstructure:"routing_prefix"`
}

// Validate checks if the ChaintracksServerConfig fields contain valid values and returns an error if any are invalid.
func (c *ChaintracksServerConfig) Validate() error {
	if err := c.ChaintracksServiceConfig.Validate(); err != nil {
		return fmt.Errorf("invalid chaintracks service config: %w", err)
	}

	const maxPort = 65535
	if c.Port == 0 || c.Port > maxPort {
		return fmt.Errorf("invalid port: %d", c.Port)
	}

	return nil
}

// DefaultChaintracksServerConfig returns a ChaintracksServerConfig with default settings for the Chaintracks server.
// Sets port to 3011 and applies the default ChaintracksServiceConfig using the mainnet BSV network.
// Intended to provide a ready-to-use configuration for starting a Chaintracks server instance.
func DefaultChaintracksServerConfig() ChaintracksServerConfig {
	return ChaintracksServerConfig{
		Port:                     3011,
		ChaintracksServiceConfig: DefaultChaintracksServiceConfig(),
	}
}
