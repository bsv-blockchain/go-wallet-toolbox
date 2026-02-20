package certifier

import (
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// ServerConfig holds configuration options for the certifier server.
type ServerConfig struct {
	Port       string
	Logger     *slog.Logger
	Originator string
	Randomizer wdk.Randomizer
}

func defaultConfig() *ServerConfig {
	return &ServerConfig{
		Port:       "8080",
		Originator: "certifier-server",
		Logger:     slog.Default(),
		Randomizer: randomizer.New(),
	}
}

// WithPort sets the server port.
func WithPort(port string) func(*ServerConfig) {
	return func(c *ServerConfig) {
		c.Port = port
	}
}

// WithLogger sets the server logger.
func WithLogger(logger *slog.Logger) func(*ServerConfig) {
	return func(c *ServerConfig) {
		c.Logger = logger
	}
}

// WithOriginator sets the originator identifier for wallet operations.
func WithOriginator(originator string) func(*ServerConfig) {
	return func(c *ServerConfig) {
		c.Originator = originator
	}
}

// WithRandomizer sets the random number generator for nonce creation.
func WithRandomizer(r wdk.Randomizer) func(*ServerConfig) {
	return func(c *ServerConfig) {
		c.Randomizer = r
	}
}
