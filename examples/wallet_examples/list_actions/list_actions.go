package main

import (
	"context"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

var (
	// DefaultLimit is the default number of actions to retrieve.
	DefaultLimit = uint32(100)

	// DefaultOffset is the default starting position for pagination.
	DefaultOffset = uint32(0)

	// DefaultOriginator specifies the originator domain or FQDN used to identify the source of the action listing request.
	// NOTE: Replace "example.com" with the actual originator domain or FQDN in real usage.
	DefaultOriginator = "example.com"

	// DefaultIncludeLabels is the default value for including labels in the response.
	DefaultIncludeLabels = true

	// DefaultLabels is the default labels to filter actions by.
	DefaultLabels = []string{}

	// DefaultLabelQueryMode is the default mode for querying labels (All or Any).
	DefaultLabelQueryMode = sdk.QueryModeAny
)

// defaultListActionsArgs creates default arguments for listing wallet actions.
// This function demonstrates how to configure the ListActionsArgs struct which controls:
// - Filtering: which labels to filter by and how to query them
// - Pagination: how many results to return and where to start
// - Data inclusion: whether to include additional metadata like labels
func defaultListActionsArgs() sdk.ListActionsArgs {
	return sdk.ListActionsArgs{
		Labels:         DefaultLabels,         // Empty labels means list all actions regardless of labels
		LabelQueryMode: DefaultLabelQueryMode, // How to query multiple labels (Any/All)
		Limit:          &DefaultLimit,         // Maximum number of actions to return (100)
		Offset:         &DefaultOffset,        // Starting position for pagination (0 = start from beginning)
		IncludeLabels:  &DefaultIncludeLabels, // Include labels associated with actions in the response
	}
}

// This example demonstrates how to list actions for the Alice wallet using default arguments.
// It shows the complete flow from wallet creation to action listing with proper error handling.
func main() {
	show.ProcessStart("List Actions")
	ctx := context.Background()
	alice := example_setup.CreateAlice()

	aliceWallet, cleanup, err := alice.CreateWallet(ctx)
	defer cleanup()
	if err != nil {
		panic(fmt.Errorf("failed to create Alice's wallet: %w", err))
	}

	show.Step("Alice", "Listing actions")
	args := defaultListActionsArgs()

	actions, err := aliceWallet.ListActions(ctx, args, DefaultOriginator)
	if err != nil {
		panic(fmt.Errorf("failed to list actions: %w", err))
	}

	show.Info("Actions", actions)
	show.ProcessComplete("List Actions")
}
