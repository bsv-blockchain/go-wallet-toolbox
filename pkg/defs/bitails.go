package defs

import "fmt"

const (
	scriptHashHistoryPageLimitMaximum = 5000
	defaultScriptHashHistoryPageLimit = 100
)

// Bitails configures the Bitails service
type Bitails struct {
	Enabled                    bool   `mapstructure:"enabled"`
	APIKey                     string `mapstructure:"api_key"`
	ScriptHashHistoryPageLimit int    `mapstructure:"script_hash_history_page_limit"`
}

// Validate checks if the Bitails configuration is valid
func (b *Bitails) Validate() error {
	if !b.Enabled {
		return nil
	}
	if b.ScriptHashHistoryPageLimit <= 0 {
		return fmt.Errorf("script hash history page limit must be greater than 0")
	}
	if b.ScriptHashHistoryPageLimit > scriptHashHistoryPageLimitMaximum {
		return fmt.Errorf("script hash history page limit must not exceed %d", scriptHashHistoryPageLimitMaximum)
	}
	return nil
}
