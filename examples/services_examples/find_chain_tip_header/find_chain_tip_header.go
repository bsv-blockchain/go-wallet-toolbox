package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// This example demonstrates retrieving the complete block header for the latest block (chain tip)
// Returns detailed blockchain metadata including hash, merkle root, difficulty, and timestamp
func main() {
	show.ProcessStart("Find Chain Tip Header")

	// Configure services for mainnet with Block Headers Service settings
	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	cfg.BHS.URL = "http://localhost:8080"
	cfg.BHS.APIKey = "..." // use default api key DefaultAppToken from the BHS service https://github.com/bsv-blockchain/block-headers-service/blob/main/config/defaults.go#L8

	// Create services instance with logging configuration
	svc := services.New(slog.Default(), cfg)
	ctx := context.Background()

	show.Step("FindChainTipHeader", "Finds the latest block header in the longest chain")

	// Retrieve complete block header data for the chain tip
	tip, err := svc.FindChainTipHeader(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to find chain tip header: %w", err))
	}

	show.Success("Fetched chain tip header")
	show.ChainTipHeaderOutput(tip)

	show.ProcessComplete("Find Chain Tip Header")
}
