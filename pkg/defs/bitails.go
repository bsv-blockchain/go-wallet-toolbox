package defs

// Bitails configures the Bitails service
type Bitails struct {
	// APIKey is the authentication key used to communicate with Bitails (if applicable)
	APIKey string `mapstructure:"api_key"`
}

// Validate checks if the Bitails configuration is valid
func (b *Bitails) Validate() error {
	return nil
}
