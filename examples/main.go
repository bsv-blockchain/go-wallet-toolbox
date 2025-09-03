package main

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
)

func main() {
	cfg, err := example_setup.LoadConfig()
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	fmt.Printf("Configuration loaded successfully\n")
	fmt.Printf("Network: %s\n", cfg.Network)
	fmt.Printf("Server URL: %s\n", cfg.ServerURL)

	setup := example_setup.CreateAlice()

	ctx := context.Background()
	_, cleanup := setup.CreateWallet(ctx)
	defer cleanup()

	fmt.Printf("Local storage initialized (or remote connected). Press Ctrl+C to exit.\n")

	// Keep the program running
	select {}
}
