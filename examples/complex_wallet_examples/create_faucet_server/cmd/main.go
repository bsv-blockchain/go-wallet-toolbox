package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/localinfra"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/server"
	"github.com/subosito/gotenv"
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
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	privKey, err := ec.NewPrivateKey()
	if err != nil {
		log.Fatalf("failed to generate ephemeral server key: %v", err)
	}
	_ = hex.EncodeToString(privKey.Serialize())

	// Initialize local GORM storage provider (no extra server)
	storageInfra, err := localinfra.CreateLocalStorage(context.Background(), cfg.Network)
	if err != nil {
		log.Fatalf("failed to init local storage: %v", err)
	}

	// Start Fiber HTTP server
	app := server.New(cfg, storageInfra.Provider)
	addr := fmt.Sprintf(":%d", cfg.Port)
	if err := app.Start(addr); err != nil {
		log.Fatalf("fiber server exited: %v", err)
	}
}
