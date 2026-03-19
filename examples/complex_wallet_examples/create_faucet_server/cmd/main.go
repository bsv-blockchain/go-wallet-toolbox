package main

import (
	"context"
	"fmt"
	"log"

	"github.com/subosito/gotenv"

	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/create_storage"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/server"
)

func main() {
	// Load .env if present
	_ = gotenv.Load(".env")
	_ = gotenv.Load("examples/complex_wallet_examples/create_faucet_server/.env")

	// Load config from environment variables
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	if err = cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	// Initialize local GORM storage provider
	provider, cleanup, err := create_storage.CreateLocalStorage(context.Background(), cfg.Network, cfg.FaucetPrivateKey)
	if err != nil {
		log.Fatalf("failed to init local storage: %v", err)
	}
	defer cleanup()

	// Start Fiber HTTP server
	app := server.New(cfg, provider)
	addr := fmt.Sprintf(":%d", cfg.Port)
	if err := app.Start(addr); err != nil {
		log.Fatalf("fiber server exited: %v", err)
	}
}
