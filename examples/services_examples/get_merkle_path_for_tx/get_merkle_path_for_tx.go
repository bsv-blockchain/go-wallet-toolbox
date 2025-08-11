package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// This example demonstrates retrieving a Merkle path for a specific transaction ID
// Merkle paths provide cryptographic proof of transaction inclusion in blocks for SPV verification
// Uses multiple blockchain data services with automatic fallback logic
// Returns the first successful result or an error if all services fail.
// https://whatsonchain.com/block-height/903321?tab=json <-- Example of a block with Merkle Path
func main() {
	show.ProcessStart("Merkle Path")

	// Define transaction ID and network for Merkle path lookup
	txID := "5a98aa7924f2a1bd171161d58122c2c870108fe367f1e335576976acaa78fd3c"
	network := defs.NetworkTestnet

	slog.SetLogLoggerLevel(slog.LevelDebug)

	// Configure services stack with default settings
	serviceCfg := defs.DefaultServicesConfig(network)
	walletServices := services.New(slog.Default(), serviceCfg)

	show.Step("Wallet-Services", fmt.Sprintf("fetching Merkle Path for txID %s", txID))

	// Retrieve Merkle path with automatic fallback across multiple services
	result, err := walletServices.MerklePath(context.Background(), txID)
	if err != nil {
		panic(fmt.Errorf("failed to get MerklePath: %w", err))
	}

	show.Success("Fetched Merkle Path")
	show.MerklePathOutput(result)
	show.ProcessComplete("Merkle Path")
}
