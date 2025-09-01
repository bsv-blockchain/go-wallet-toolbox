package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
)

func main() {
	setupConsoleLogging()

	ctx := context.Background()

	cfg, err := example_setup.LoadConfig()
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	// Create storage as a Go object (not a server)
	fmt.Printf("Setting up local storage infrastructure\n")
	storage, err := example_setup.CreateLocalStorage(ctx, cfg.Network, cfg.ServerPrivateKey)
	if err != nil {
		panic(fmt.Errorf("failed to create local storage: %w", err))
	}

	// Set global storage so wallet examples can access it
	example_setup.SetGlobalStorage(storage)

	fmt.Printf("Local storage created successfully\n")
	fmt.Printf("Storage infrastructure ready. You can now run wallet examples.\n")
	fmt.Printf("Press Ctrl+C to exit.\n")

	// Keep the program running
	select {}
}

func setupConsoleLogging() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)

	slog.Info("Console logging initialized successfully")
}
