package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// This example demonstrates retrieving the current block height using multiple blockchain data services
// When you call srv.CurrentHeight() the services stack:
// 1. Tries Block Headers Service (/chain/tip/longest) first
// 2. Falls back to WhatsOnChain (/chain/info) if BHS fails
// 3. Falls back to Bitails (/network/info) if WoC fails
// 4. Returns the first non-zero height it obtains
func main() {
	show.ProcessStart("Get Height")

	// Configure services for mainnet with default settings
	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	cfg.BHS.APIKey = "..." // use default api key DefaultAppToken from the BHS service https://github.com/bsv-blockchain/block-headers-service/blob/main/config/defaults.go#L8

	// Create services instance with logging and fallback configuration
	srv := services.New(slog.Default(), cfg)
	show.Step("Wallet-Services", "fetching main-chain height (BHS → WoC → Bitails fallback)")

	// Retrieve current block height with automatic fallback (BHS → WoC → Bitails)
	height, err := srv.CurrentHeight(context.Background())
	if err != nil {
		panic(fmt.Errorf("failed to get height: %w", err))
	}

	show.Success("Fetched chain tip height")
	show.CurrentHeightOutput(height)
	show.ProcessComplete("Get Height")
}
