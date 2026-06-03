package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/bsv-blockchain/certifier-server-example/internal/config"
	"github.com/bsv-blockchain/certifier-server-example/internal/server"
)

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.LoadConfig("", logger)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to load config", "error", err)
		os.Exit(1)
	}

	if err = cfg.Validate(); err != nil {
		logger.ErrorContext(ctx, "Invalid configuration", "error", err)
		os.Exit(1)
	}

	srv, err := server.New(cfg, logger)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create server", "error", err)
		os.Exit(1)
	}

	if err := srv.Start(); err != nil {
		logger.ErrorContext(ctx, "Server failed", "error", err)
		os.Exit(1)
	}
}
