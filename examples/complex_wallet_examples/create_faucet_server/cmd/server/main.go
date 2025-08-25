package main

import (
	"context"
	"fmt"
	"log"
	"os"

	exconfig "github.com/bsv-blockchain/go-wallet-toolbox/examples/complex_wallet_examples/create_faucet_server/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/complex_wallet_examples/create_faucet_server/internal/server"
	intconfig "github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
	"github.com/subosito/gotenv"
)

func main() {
	// Load .env if present
	_ = gotenv.Load(".env")
	_ = gotenv.Load("examples/complex_wallet_examples/create_faucet_server/.env")

	// Use shared Loader with faucet defaults
	loader := intconfig.NewLoader(exconfig.Defaults, "")
	cfg, err := loader.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	// Start Wallet Toolbox infra (GORM local storage + JSON-RPC server)
	go func() {
		_ = os.Setenv("SERVER_PRIVATE_KEY", cfg.ServerPrivateKey)
		_ = os.Setenv("BSV_NETWORK", string(cfg.Network))

		srv, err := infra.NewServer(
			context.Background(),
			infra.WithEnvPrefix(""),
		)
		if err != nil {
			log.Fatalf("infra init failed: %v", err)
		}
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("infra server exited: %v", err)
		}
	}()

	// Start Fiber HTTP server
	app := server.New(cfg)
	addr := fmt.Sprintf(":%d", cfg.Port)
	if err := app.Start(addr); err != nil {
		log.Fatalf("fiber server exited: %v", err)
	}
}
