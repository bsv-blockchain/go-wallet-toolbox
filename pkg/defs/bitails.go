package defs

import (
	"fmt"
	"net/url"
)

// Bitails configures the Bitails service
type Bitails struct {
	// APIKey is the authentication key used to communicate with Bitails (if applicable)
	APIKey string `mapstructure:"api_key"`

	// URL overrides the default Bitails endpoint (useful for testing/mocks)
	URL string `mapstructure:"url"`
}

// Validate checks if the Bitails configuration is valid
func (b *Bitails) Validate() error {
	if b == nil || b.URL == "" {
		return nil
	}

	parsedURL, err := url.Parse(b.URL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s - must be http or https", parsedURL.Scheme)
	}

	return nil
}
