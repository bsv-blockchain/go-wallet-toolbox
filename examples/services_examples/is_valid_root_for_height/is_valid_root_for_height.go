package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/chainhash"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// This example demonstrates validating if a Merkle root is valid for a specific block height
// Essential for SPV implementations to verify transaction inclusion in blocks
func main() {
	const (
		height = uint32(903321)
		// https://whatsonchain.com/block-height/903321?tab=json
		rootHex = "559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd"
	)

	show.ProcessStart("Is Valid Root For Height")

	// Configure services for mainnet with Block Headers Service settings
	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	cfg.BHS.APIKey = "..." // use default api key DefaultAppToken from the BHS service https://github.com/bsv-blockchain/block-headers-service/blob/main/config/defaults.go#L8

	// Create services instance for blockchain data access
	srv := services.New(slog.Default(), cfg)

	// Convert hex string to proper hash format
	root, err := chainhash.NewHashFromHex(rootHex)
	if err != nil {
		panic(fmt.Errorf("failed to parse root hex %s: %w", rootHex, err))
	}

	show.Step("Wallet-Services", fmt.Sprintf("checking if root %s is valid for height %d", rootHex, height))

	// Validate Merkle root against the specified block height
	ok, err := srv.IsValidRootForHeight(context.Background(), root, height)
	if err != nil {
		panic(fmt.Errorf("IsValidRootForHeight failed: %w", err))
	}

	show.Success("Checked if root is valid for height")
	show.IsValidRootForHeightOutput(height, rootHex, ok)
	show.ProcessComplete("Is Valid Root For Height")
}
