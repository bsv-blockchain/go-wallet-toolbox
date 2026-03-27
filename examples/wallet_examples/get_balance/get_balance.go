package main

import (
	"context"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/must"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

var (
	// Limit is the number of outputs to retrieve per page for balance calculation.
	Limit = uint32(100)

	// Originator specifies the originator domain or FQDN used to identify the source of the balance request.
	// NOTE: Replace "example.com" with the actual originator domain or FQDN in real usage.
	Originator = "example.com"

	// Basket is the basket to get balance from (holds automatically managed "change").
	Basket = "default"
)

// This example demonstrates how to calculate the balance of a wallet by summing
// all satoshis from outputs in the default basket using pagination.
func main() {
	show.ProcessStart("Get Wallet Balance")
	ctx := context.Background()

	alice := example_setup.CreateAlice()

	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	show.Step("Alice", "Calculating wallet balance")

	var balance uint64
	var offset uint32

	for {
		// Retrieve outputs from the 'default' basket with pagination
		args := sdk.ListOutputsArgs{
			Basket: Basket,
			Limit:  &Limit,
			Offset: &offset,
		}

		outputs, err := aliceWallet.ListOutputs(ctx, args, Originator)
		if err != nil {
			panic(fmt.Errorf("failed to list outputs: %w", err))
		}

		// Sum the satoshis from all outputs in this page
		for _, output := range outputs.Outputs {
			balance += output.Satoshis
		}

		// Update offset for next page
		offset += must.ConvertToUInt32(len(outputs.Outputs))

		// Break if we've retrieved all outputs
		if len(outputs.Outputs) < int(Limit) {
			break
		}
	}

	show.Info("Total Balance (satoshis)", balance)
	show.ProcessComplete("Get Wallet Balance")
}
