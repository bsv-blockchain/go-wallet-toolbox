package defs

import "fmt"

// ChaintracksClient configures the ChaintracksClient service
type ChaintracksClient struct {
	Enabled bool   `mapstructure:"enabled"`
	Mode    string `mapstructure:"mode"` // "remote" or "embedded"

	RemoteURL string `mapstructure:"remote_url"` // remote  mode config
	// TODO: add embedded config
}

// Validate checks if the ChaintracksClient configuration is valid
func (c *ChaintracksClient) Validate() error {
	if !c.Enabled {
		return nil
	}

	switch c.Mode {
	case "remote":
		if c.RemoteURL == "" {
			return fmt.Errorf("remote_url is required when mode is 'remote'")
		}
	case "embedded":
	// TODO: embedded mode config validation will be added later
	case "":
		return fmt.Errorf("mode is required when chaintracks is enabled")
	default:
		return fmt.Errorf("invalid chaintracks mode: %s (must be 'remote' or 'embedded')", c.Mode)
	}

	return nil
}
