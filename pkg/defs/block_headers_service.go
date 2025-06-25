package defs

import "fmt"

// BHS represents the configuration for a BHS (Backend HTTP Service) client.
// It includes the base URL of the service and an optional API key for authentication.
type BHS struct {
	URL    string `mapstructure:"url"`     // URL is the base endpoint of the BHS service.
	APIKey string `mapstructure:"api_key"` // APIKey is the authentication key used for accessing the BHS service.
}

// Validate checks if the BHS configuration is valid.
// It ensures that both the URL and APIKey fields are not empty.
// Returns an error if any required field is missing.
func (b *BHS) Validate() error {
	if len(b.APIKey) == 0 {
		return fmt.Errorf("validation failed: APIKey must not be empty")
	}
	if len(b.URL) == 0 {
		return fmt.Errorf("validation failed: URL must not be empty")
	}

	return nil
}
