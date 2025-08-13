package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// This example demonstrates how to get the status of a UTXO by script hash.
// The services package is used to interact with the WhatsOnChain API.
// The scriptHash is the script hash of the UTXO you want to get the status of.
// The result is a list of UTXOs with their status.
func main() {
	const scriptHash = "b3005d46af31c4b5675b73c17579b7bd366dfe10635b7b43ac111aea5226efb6"

	show.ProcessStart("Get UTXOs By ScriptHash")
	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	srv := services.New(slog.Default(), cfg)

	show.Step("Wallet-Services", fmt.Sprintf("fetching UTXOs from WhatsOnChain for scriptHash: %s", scriptHash))

	ctx := context.Background()
	result, err := srv.GetUtxoStatus(ctx, scriptHash, nil)
	if err != nil {
		show.Error(fmt.Sprintf("failed to fetch UTXOs: %v", err))
		return
	}

	show.Success(fmt.Sprintf("Success, found %d UTXOs", len(result.Details)))
	show.GetUtxoStatusOutput(result)
	show.ProcessComplete("Get UTXOs By ScriptHash")
}
