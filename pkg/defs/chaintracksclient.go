package defs

import "fmt"

// ChaintracksClientMode represents available modes for running chaintracks.
type ChaintracksClientMode string

const (
	// ChaintracksClientModeEmbedded represents embedded mode for running chaintracks.
	ChaintracksClientModeEmbedded ChaintracksClientMode = "embedded"
	// ChaintracksClientModeRemote represents remote mode for running chaintracks.
	ChaintracksClientModeRemote ChaintracksClientMode = "remote"
)

// ChaintracksClient configures the ChaintracksClient service
type ChaintracksClient struct {
	Enabled bool                  `mapstructure:"enabled"`
	Mode    ChaintracksClientMode `mapstructure:"mode"` // "remote" or "embedded"

	// remote mode config
	RemoteURL string `mapstructure:"remote_url"` // remote  mode config

	// embedded mode config
	StoragePath   string `mapstructure:"storage_path"`
	BootstrapURL  string `mapstructure:"bootstrap_url"`
	BootstrapMode string `mapstructure:"bootstrap_mode"` // "api" or "cdn"

	// P2P settings (embedded mode)
	P2PNetwork     string `mapstructure:"p2p_network"`
	P2PStoragePath string `mapstructure:"p2p_storage_path"`
}

// Validate checks if the ChaintracksClient configuration is valid
func (c *ChaintracksClient) Validate() error {
	if !c.Enabled {
		return nil
	}

	switch c.Mode {
	case ChaintracksClientModeRemote:
		if c.RemoteURL == "" {
			return fmt.Errorf("remote_url is required when mode is 'remote'")
		}
	case ChaintracksClientModeEmbedded:
		if c.P2PNetwork == "" {
			return fmt.Errorf("p2p_network is required when mode is 'embedded'")
		}

		// Validate P2PNetwork value
		switch c.P2PNetwork {
		case "main", "test", "stn":
			// valid
		default:
			return fmt.Errorf("invalid p2p_network: %s (must be 'main', 'test', or 'stn')", c.P2PNetwork)
		}
	case "":
		return fmt.Errorf("mode is required when chaintracks is enabled")
	default:
		return fmt.Errorf("invalid chaintracks mode: %s (must be 'remote' or 'embedded')", c.Mode)
	}

	return nil
}
