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

	listenForTipHeaders(server, ctx)
	listenForReorgs(server, ctx)

	if err := server.ListenAndServe(ctx); err != nil {
		panic(err)
	}
}

func listenForTipHeaders(server *chaintracks.Server, ctx context.Context) {
	newTipHeadersChan, unsubscribe := server.Service.SubscribeHeaders()

	go func() {
		for {
			select {
			case header := <-newTipHeadersChan:
				slog.Default().Info("New tip header received", slog.Uint64("height", uint64(header.Height)), slog.String("hash", header.Hash))
			case <-ctx.Done():
				unsubscribe()
				return
			}
		}
	}()
}

func listenForReorgs(server *chaintracks.Server, ctx context.Context) {
	reorgChan, unsubscribe := server.Service.SubscribeReorgs()

	go func() {
		for {
			select {
			case reorg := <-reorgChan:
				slog.Default().Info("Reorg detected",
					slog.Uint64("new_tip_height", uint64(reorg.NewTip.Height)),
					slog.String("new_tip_hash", reorg.NewTip.Hash),
					slog.Uint64("old_tip_height", uint64(reorg.OldTip.Height)),
					slog.String("old_tip_hash", reorg.OldTip.Hash),
				)
			case <-ctx.Done():
				unsubscribe()
				return
			}
		}
	}()
}
