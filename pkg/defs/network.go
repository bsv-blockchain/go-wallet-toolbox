package defs

import (
	"fmt"
)

// BSVNetwork represents the Bitcoin SV network type (mainnet or testnet)
type BSVNetwork string

// BSVNetwork constants for the different Bitcoin SV network types
const (
	NetworkMainnet BSVNetwork = "main"
	NetworkTestnet BSVNetwork = "test"
)

// ParseBSVNetworkStr will parse the given string and return the corresponding BSVNetwork type or an error
func ParseBSVNetworkStr(network string) (BSVNetwork, error) {
	return parseEnumCaseInsensitive(network, NetworkMainnet, NetworkTestnet)
}

// Validate checks if the value underneath is within valid BSVNetwork values.
func (n BSVNetwork) Validate() error {
	switch n {
	case NetworkMainnet, NetworkTestnet:
		return nil
	default:
		return fmt.Errorf("unsupported BSV network: %s", n)
	}
}
