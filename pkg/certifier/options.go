package certifier

import (
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

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

func WithPort(port string) func(*ServerConfig) {
	return func(c *ServerConfig) {
		c.Port = port
	}
}

func WithLogger(logger *slog.Logger) func(*ServerConfig) {
	return func(c *ServerConfig) {
		c.Logger = logger
	}
}

func WithOriginator(originator string) func(*ServerConfig) {
	return func(c *ServerConfig) {
		c.Originator = originator
	}
}

func WithRandomizer(r wdk.Randomizer) func(*ServerConfig) {
	return func(c *ServerConfig) {
		c.Randomizer = r
	}
}
