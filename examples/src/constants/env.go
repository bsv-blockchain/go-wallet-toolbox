package constants

import "github.com/4chain-ag/go-wallet-toolbox/pkg/defs"

// Environment contains environment configuration
type Environment struct {
	Network   defs.BSVNetwork
	ServerURL string
	DevKeys   DevKeys
}

// GetEnv returns the environment configuration for the given network
func GetEnv(network defs.BSVNetwork) Environment {
	return Environment{
		Network:   network,
		ServerURL: "http://localhost:8100",
		DevKeys:   GetDevKeys(),
	}
}
