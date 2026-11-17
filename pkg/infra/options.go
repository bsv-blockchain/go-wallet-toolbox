package infra

import (
	"context"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/keyderiver"
)

// KeyDeriverFactory opens the root-key deriver when hsm.enabled is set.
// The toolbox does not import the HSM module. The HSM server command supplies
// this factory so a downstream go.mod that imports pkg/infra does not need it.
type KeyDeriverFactory func(ctx context.Context, cfg HSMConfig, logger *slog.Logger) (keyderiver.Deriver, error)

// Options is the parameters for initializing the "infra" server
type Options struct {
	EnvPrefix         string
	ConfigFile        string
	Logger            *slog.Logger
	KeyDeriverFactory KeyDeriverFactory
}

func defaultOptions() Options {
	return Options{
		EnvPrefix:  "INFRA",
		ConfigFile: "",
		Logger:     nil,
	}
}

// InitOption is a function that sets a parameter for initializing the "infra" server
type InitOption func(*Options)

// WithEnvPrefix sets the environment variable prefix for the "infra" server, all environment variables will be prefixed with this:
// e.g. "INFRA_HTTP_PORT=8100"
func WithEnvPrefix(prefix string) InitOption {
	return func(o *Options) {
		o.EnvPrefix = prefix
	}
}

// WithConfigFile sets the configuration file for the "infra" server, the configuration file is in YAML format
func WithConfigFile(file string) InitOption {
	return func(o *Options) {
		o.ConfigFile = file
	}
}

// WithLogger sets the logger for the "infra" server
func WithLogger(logger *slog.Logger) InitOption {
	return func(o *Options) {
		o.Logger = logger
	}
}

// WithKeyDeriverFactory supplies the opener used when hsm.enabled is set.
// Without it, an enabled HSM configuration fails at startup. The Securosys
// opener lives in hsm/cmd/infra so this module does not depend on the HSM module.
func WithKeyDeriverFactory(factory KeyDeriverFactory) InitOption {
	return func(o *Options) {
		o.KeyDeriverFactory = factory
	}
}
