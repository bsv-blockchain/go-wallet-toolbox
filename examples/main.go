package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
)

func main() {
	setupConsoleLogging()

	cfg, err := example_setup.LoadConfig()
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	fmt.Printf("Configuration loaded successfully\n")
	fmt.Printf("Network: %s\n", cfg.Network)
	fmt.Printf("Server URL: %s\n", cfg.ServerURL)
	fmt.Printf("Examples are ready to run!\n")
	fmt.Printf("Each example will create its own local storage.\n")
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
