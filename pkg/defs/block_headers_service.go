package defs

import "fmt"

type BHS struct {
	URL    string `mapstructure:"url"`
	APIKey string `mapstructure:"api_key"`
}

func (b *BHS) Validate() error {
	if len(b.APIKey) == 0 {
		return fmt.Errorf("validation failed: APIKey must not be empty")
	}
	if len(b.URL) == 0 {
		return fmt.Errorf("validation failed: URL must not be empty")
	}

	return nil
}
