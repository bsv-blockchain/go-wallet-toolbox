package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// This example demonstrates how to get a block header from a block hash.
// It fetches the block header for a given blockhash and prints the header details.
func main() {
	const blockHash = "000000000000000004a288072ebb35e37233f419918f9783d499979cb6ac33eb" // example block hash

	show.ProcessStart("Hash To Header")

	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)

	srv := services.New(slog.Default(), cfg)

	show.Step("Wallet-Services", fmt.Sprintf("fetching block header for hash %s", blockHash))
	header, err := srv.HashToHeader(context.Background(), blockHash)
	if err != nil {
		panic(fmt.Errorf("failed to get header for hash: %w", err))
	}

	show.Success("Fetched block header from hash")
	show.ChainTipHeaderOutput(header)
	show.ProcessComplete("Hash To Header")
}
