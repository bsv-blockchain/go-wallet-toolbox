package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
)

func main() {
	setupConsoleLogging()

	ctx := context.Background()

	cfg, err := example_setup.LoadConfig()
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	var storageCleanup func()

	if cfg.ServerURL != "" {
		// Use remote server when URL is provided
		fmt.Printf("Connecting to remote server: %s\n", cfg.ServerURL)
		client, clientCleanup, err := storage.NewClient(cfg.ServerURL)
		if err != nil {
			panic(fmt.Errorf("failed to connect to server: %w", err))
		}

		fmt.Printf("Testing connection to remote server...\n")
		_, err = client.MakeAvailable(ctx)
		if err != nil {
			panic(fmt.Errorf("failed to ping server: %w", err))
		}
		fmt.Printf("✓ Successfully connected to remote storage server\n")

		storageCleanup = clientCleanup
	} else {
		// Use local storage when no URL is provided
		fmt.Printf("Setting up local storage infrastructure\n")
		storage, err := example_setup.CreateLocalStorage(ctx, cfg.Network, cfg.ServerPrivateKey)
		if err != nil {
			panic(fmt.Errorf("failed to create local storage: %w", err))
		}
		storageCleanup = func() {
			if storage.Monitor != nil {
				storage.Monitor.Stop()
			}
		}
		fmt.Printf("Local storage will be created in: ./storage.sqlite\n")
	}

	defer storageCleanup()

	select {} // keep the program running indefinitely
}

func setupConsoleLogging() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)

	slog.Info("Console logging initialized successfully")
}
