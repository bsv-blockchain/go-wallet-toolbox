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

	// DefaultOriginator specifies the originator domain or FQDN used to identify the source of the request.
	DefaultOriginator = "example.com"

	// DefaultIncludeLabels determines whether to include labels in the response
	DefaultIncludeLabels = true

	// DefaultUnfail determines whether to request unfail processing for returned failed actions
	DefaultUnfail = false
)

// defaultListFailedActionsArgs creates default arguments for listing failed wallet actions
func defaultListFailedActionsArgs() sdk.ListActionsArgs {
	return sdk.ListActionsArgs{
		Limit:         &DefaultLimit,
		Offset:        &DefaultOffset,
		IncludeLabels: &DefaultIncludeLabels,
	}
}

// This example demonstrates how to list failed actions for the Alice wallet
func main() {
	show.ProcessStart("List Failed Actions")

	ctx := context.Background()

	// Create Alice's wallet instance
	alice := example_setup.CreateAlice()

	// Create the wallet interface and establish database connection
	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	show.Step("Alice", "Listing failed actions")

	// Configure pagination and filtering parameters
	args := defaultListFailedActionsArgs()
	show.Info("ListFailedActionsArgs", args)
	show.Info("Unfail", DefaultUnfail)
	show.Separator()

	// Retrieve paginated list of failed wallet actions
	actions, err := aliceWallet.ListFailedActions(ctx, args, DefaultUnfail, DefaultOriginator)
	if err != nil {
		panic(fmt.Errorf("failed to list failed actions: %w", err))
	}

	show.Info("FailedActions", actions)
	show.ProcessComplete("List Failed Actions")
}
