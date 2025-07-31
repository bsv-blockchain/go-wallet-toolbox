package main

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/utils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// Add beef hex directly here or use the txid to get the beef hex from the API
// BEEF (Background Evaluation Extended Format) contains transaction data with merkle proofs
var beef = ""

// The txid is the transaction id of the transaction to internalize
// This should be the transaction ID received from a testnet faucet
var txID = "15f47f2db5f26469c081e8d80d91a4b0f06e4a97abcc022b0b5163ac5f6cc0c8"

// To internalize a transaction from the faucet, you need to pass the txid of the transaction to internalize
// Use the faucet_address example to get the user address and follow the instructions to fund the address from the faucet
func main() {
	show.ProcessStart("Faucet Transaction Internalization")

	ctx := context.Background()

	show.Step("Alice", "Creating wallet and setting up environment")

	// Create Alice's wallet instance
	alice := example_setup.CreateAlice()

	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	// Fetch transaction data in BEEF format if not provided directly
	if beef == "" {
		var err error
		
		// Get complete transaction data from WhatsonChain API
		beef, err = utils.WocAPIGetBeefForTX(defs.NetworkTestnet, txID)
		if err != nil {
			panic(fmt.Errorf("failed to get beef for tx: %w", err))
		}

		show.Step("Alice", "Retrieving BEEF data for transaction")
		show.Transaction(txID)
	}

	show.Step("Alice", "Internalizing transaction from faucet")

	// This method will internalize the transaction from the faucet into the wallet database
	err := example_setup.InternalizeFromFaucet(ctx, beef, aliceWallet)
	if err != nil {
		panic(fmt.Errorf("failed to internalize tx: %w", err))
	}

	show.Success("Transaction internalized successfully")
	show.ProcessComplete("Faucet Transaction Internalization")

}
