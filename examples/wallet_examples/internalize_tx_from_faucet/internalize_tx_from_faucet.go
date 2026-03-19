package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/chainhash"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// The txID is the transaction ID of the transaction to internalize
// Pass in your txID from the faucet_address example
var txID = "" // example: 15f47f2db5f26469c081e8d80d91a4b0f06e4a97abcc022b0b5163ac5f6cc0c8

// To internalize a transaction from the faucet, you need to pass the txid of the transaction to internalize
// Use the faucet_address example to get the user address and follow the instructions to fund the address from the faucet
func main() {
	show.ProcessStart("Faucet Transaction Internalization")
	ctx := context.Background()

	if txID == "" {
		panic(fmt.Errorf("txID must be provided"))
	}

	txIDHash, err := chainhash.NewHashFromHex(txID)
	if err != nil {
		panic(fmt.Errorf("invalid txID: %w", err))
	}

	show.Step("Alice", "Creating wallet and setting up environment")
	alice := example_setup.CreateAlice()

	// Create the wallet interface and establish database connection
	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	show.Step("Alice", "Retrieving transaction data")
	show.Transaction(txID)

	// Get the transactionHex from the txID
	srv := services.New(slog.Default(), defs.DefaultServicesConfig(alice.Environment.BSVNetwork))
	show.Step("Wallet-Services", fmt.Sprintf("fetching BEEF from services for txID: %q", txID))
	beef, err := srv.GetBEEF(ctx, txID, nil)
	if err != nil {
		panic(fmt.Errorf("failed to get BEEF for txID %s: %w", txID, err))
	}

	atomicBeef, err := beef.AtomicBytes(txIDHash)
	if err != nil {
		panic(fmt.Errorf("failed to get atomic bytes for txID %s: %w", txID, err))
	}

	show.Step("Alice", "Internalizing transaction from faucet")

	// This method will internalize the transaction from the faucet into the wallet database
	err = example_setup.InternalizeFromFaucet(ctx, atomicBeef, aliceWallet)
	if err != nil {
		panic(fmt.Errorf("failed to internalize tx: %w", err))
	}

	show.Success("Transaction internalized successfully")
	show.ProcessComplete("Faucet Transaction Internalization")
}
