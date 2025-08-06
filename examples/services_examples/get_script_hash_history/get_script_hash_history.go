package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// This example demonstrates how to get the history of a script hash.
func main() {
	show.ProcessStart("Script Hash History")

	scriptHash := "c79e8d823c1ce9b80c9c340a389409f489989800044466c9d05bfef12c472232"
	network := defs.NetworkMainnet

	cfg := defs.DefaultServicesConfig(network)
	srv := services.New(slog.Default(), cfg)

	show.Step("Wallet-Services", fmt.Sprintf("fetching script history for scripthash %s", scriptHash))
	history, err := srv.GetScriptHashHistory(context.Background(), scriptHash)
	if err != nil {
		panic(fmt.Errorf("failed to fetch script history: %w", err))
	}

	show.Success("Fetched Script Hash History")
	show.ScriptHashHistoryOutput(history)
	show.ProcessComplete("Script Hash History")
}
