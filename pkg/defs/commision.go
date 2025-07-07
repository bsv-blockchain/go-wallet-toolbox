package defs

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// Commission represents the commission configuration for a storage provider.
// If satoshis is greater than 0, it means that the commission is enabled.
type Commission struct {
	Satoshis  uint64               `mapstructure:"satoshis"`
	PubKeyHex primitives.PubKeyHex `mapstructure:"pub_key_hex"`
}

// Enabled checks if the commission is enabled.
func (c *Commission) Enabled() bool {
	return c.Satoshis > 0
}

// Validate double checks if under the Type is a valid enum, and checks if the value is valid.
func (c *Commission) Validate() error {
	if !c.Enabled() {
		return nil
	}

	if err := c.PubKeyHex.Validate(); err != nil {
		return fmt.Errorf("invalid pubkey: %w", err)
	}

	return nil
}

// DefaultCommission returns a default commission configuration - disabled.
func DefaultCommission() Commission {
	return Commission{
		Satoshis:  0,
		PubKeyHex: "",
	}
}
