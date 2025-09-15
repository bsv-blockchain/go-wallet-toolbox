package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// This example demonstrates retrieving raw transaction data for a specific transaction ID
func main() {
	show.ProcessStart("Raw Transaction from WhatsOnChain and Bitails")

	// Define transaction ID and network for raw transaction lookup
	txID := "9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6"
	network := defs.NetworkMainnet

	// Configure services with default settings for multiple providers
	cfg := defs.DefaultServicesConfig(network)
	srv := services.New(slog.Default(), cfg)

	show.Step("Wallet-Services", fmt.Sprintf("fetching RawTx for txID %s using WhatsOnChain and Bitails", txID))

	// Retrieve raw transaction with automatic fallback across services
	rawTx, err := srv.RawTx(context.Background(), txID)
	if err != nil {
		panic(fmt.Errorf("failed to fetch raw transaction: %w", err))
	}

	show.Success("Success, Fetched Raw Transaction")
	show.RawTxOutput(&rawTx)
	show.ProcessComplete(fmt.Sprintf("Raw Transaction fetching completed for txID %s", txID))
}
