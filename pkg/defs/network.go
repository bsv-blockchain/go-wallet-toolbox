package defs

import (
	"fmt"
)

// BSVNetwork represents the Bitcoin SV network a wallet/service instance runs against.
type BSVNetwork string

// BSVNetwork constants for the supported Bitcoin SV networks.
//
// main/test are the public production and public test networks. ttn/tstn are the
// Teranode scaling networks:
//
//	ttn  - Teranode Test Net: a public scaling test network.
//	tstn - Teranode Scaling Test Net: a private, per-deployment scaling test network
//	       that runs only Arcade (broadcast + merkle proofs) and ChainTracks (headers);
//	       it has no public WhatsOnChain / block-explorer service. Its endpoints are not
//	       public and are supplied at runtime via environment variables (see networkconfig.go).
//
// ttn and tstn are testnet-based: address encoding, key derivation and overlay network
// selection all map them to the testnet parameters.
const (
	NetworkMainnet BSVNetwork = "main"
	NetworkTestnet BSVNetwork = "test"
	NetworkTTN     BSVNetwork = "ttn"
	NetworkTSTN    BSVNetwork = "tstn"
)

// ParseBSVNetworkStr will parse the given string and return the corresponding BSVNetwork type or an error
func ParseBSVNetworkStr(network string) (BSVNetwork, error) {
	return parseEnumCaseInsensitive(network, NetworkMainnet, NetworkTestnet, NetworkTTN, NetworkTSTN)
}

// Validate checks if the value underneath is within valid BSVNetwork values.
func (n BSVNetwork) Validate() error {
	switch n {
	case NetworkMainnet, NetworkTestnet, NetworkTTN, NetworkTSTN:
		return nil
	default:
		return fmt.Errorf("unsupported BSV network: %s", n)
	}
}

// IsTeranode reports whether the network is one of the Teranode scaling networks (ttn/tstn).
// These networks broadcast through Arcade and source headers from ChainTracks.
func (n BSVNetwork) IsTeranode() bool {
	return n == NetworkTTN || n == NetworkTSTN
}

// IsTestnetBased reports whether the network uses testnet address/key parameters.
// This is true for test, ttn and tstn - only main uses mainnet parameters.
func (n BSVNetwork) IsTestnetBased() bool {
	return n != NetworkMainnet
}
