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

	// DefaultOriginator specifies the originator domain or FQDN used to identify the source of the action listing request.
	// NOTE: Replace "example.com" with the actual originator domain or FQDN in real usage.
	DefaultOriginator = "example.com"

	// DefaultIncludeLabels determines whether to include labels in the response
	DefaultIncludeLabels = true
)

// defaultListActionsArgs creates default arguments for listing wallet actions
func defaultListActionsArgs() sdk.ListActionsArgs {
	return sdk.ListActionsArgs{
		Limit:         &DefaultLimit,         // Maximum number of actions to return
		Offset:        &DefaultOffset,        // Starting position for pagination
		IncludeLabels: &DefaultIncludeLabels, // Include labels with actions
	}
}

// This example demonstrates how to list actions for the Alice wallet
func main() {
	show.ProcessStart("List Actions")

	ctx := context.Background()

	// Create Alice's wallet instance
	alice := example_setup.CreateAlice()

	// Create the wallet interface and establish database connection
	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	show.Step("Alice", "Listing actions")

	// Configure pagination and filtering parameters
	args := defaultListActionsArgs()
	show.Info("ListActionsArgs", args)
	show.Separator()

	// Retrieve paginated list of wallet actions
	actions, err := aliceWallet.ListActions(ctx, args, DefaultOriginator)
	if err != nil {
		panic(fmt.Errorf("failed to list actions: %w", err))
	}

	show.Info("Actions", actions)
	show.ProcessComplete("List Actions")
}
