package main

import (
	"context"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

var (
	// DefaultLimit is the default number of actions to retrieve
	DefaultLimit = uint32(100)
	// DefaultOffset is the default starting position for pagination
	DefaultOffset = uint32(0)
	// DefaultOriginator is the originator domain or FQDN that is allowed to use this permission.
	DefaultOriginator = "originator"
)

func defaultListActionsArgs() sdk.ListActionsArgs {
	return sdk.ListActionsArgs{
		Limit:         &DefaultLimit,  // Maximum number of actions to return
		Offset:        &DefaultOffset, // Starting position for pagination
		IncludeLabels: nil,            // Include labels in the response
	}
}

// This example demonstrates how to list actions for the Alice wallet using default arguments.
// It shows the complete flow from wallet creation to action listing with proper error handling.
func main() {
	show.ProcessStart("List Actions")
	ctx := context.Background()
	alice := example_setup.CreateAlice()

	aliceWallet, cleanup, err := alice.CreateWallet(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to create Alice's wallet: %w", err))
	}

	defer cleanup()
	show.Step("Alice", "Listing actions")
	args := defaultListActionsArgs()

	actions, err := aliceWallet.ListActions(ctx, args, DefaultOriginator)
	if err != nil {
		panic(fmt.Errorf("failed to list actions: %w", err))
	}

	show.Info("Actions", actions)
	show.ProcessComplete("List Actions")
}
