package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/bsv-blockchain/go-chaintracks/config"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracksclient"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// To Run with the remote either provide proper url to the remote chaintracks server
	// or run one by yourself for example from https://github.com/bsv-blockchain/go-chaintracks
	cfg := &config.Config{
		Mode: config.ModeRemote,
		URL:  "http://localhost:3011",
	}
	// For Embedded mode
	//  cfg := &config.Config{
	//		Mode:         config.ModeEmbedded,
	//		StoragePath:  "~/.chaintracks", // where to store headers locally
	//		BootstrapURL: "http://localhost:3011", // optional: where to bootstrap headers form
	//	}

	svc, err := chaintracksclient.New(logger, cfg)
	if err != nil {
		logger.Error("failed to create service", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = svc.Start(ctx, chaintracksclient.Callbacks{
		OnReorg: func(event *chaintracks.ReorgEvent) error {
			logger.Info("new reorg event received",
				"depth", event.Depth,
				"tip", event.NewTip,
				"orhpaned hashes", event.OrphanedHashes,
			)
			return nil
		},
		OnTip: func(header *chaintracks.BlockHeader) error {
			logger.Info("new tip received",
				"height", header.Height,
				"hash", header.Hash.String(),
			)
			return nil
		},
	})
	if err != nil {
		logger.Error("failed to start service", "err", err)
		os.Exit(1)
	}

	logger.Info("listening for reorgs... press Ctrl+C to exit")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down")
}
