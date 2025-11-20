package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks"
)

const (
	envPrefix  = "CHAINTRACKS"
	configFile = "chaintracks-config.yaml"
)

func main() {
	ctx := context.Background()
	loader := config.NewLoader(defs.DefaultChaintracksServerConfig, envPrefix)

	// optionally load from config file if it exists
	_, err := os.Stat(configFile)
	if !os.IsNotExist(err) {
		err := loader.SetConfigFilePath(configFile)
		if err != nil {
			panic(fmt.Errorf("failed to set config file path: %w", err))
		}
		slog.Default().Info("loading config from file", "file", configFile)
	} else {
		slog.Default().Info("config file not found, proceeding with environment variables and defaults")
	}

	cfg, err := loader.Load()
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	err = cfg.Validate()
	if err != nil {
		panic(fmt.Errorf("config validation failed: %w", err))
	}

	logger := chaintracks.MakeLogger(cfg.Logging)

	server, err := chaintracks.NewServer(logger, cfg)
	if err != nil {
		panic(err)
	}

	if err := server.ListenAndServe(ctx); err != nil {
		panic(err)
	}
}
