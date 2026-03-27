package main

import (
	"context"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

var (
	// DefaultLimit is the default number of outputs to retrieve.
	DefaultLimit = uint32(100)

	// DefaultOffset is the default starting position for pagination.
	DefaultOffset = uint32(0)

	// DefaultOriginator specifies the originator domain or FQDN used to identify the source of the output listing request.
	// NOTE: Replace "example.com" with the actual originator domain or FQDN in real usage.
	DefaultOriginator = "example.com"

	// DefaultIncludeLabels is the default value for including labels in the response.
	DefaultIncludeLabels = true

	// DefaultBasket is the default basket to list outputs from, if empty it will list from all baskets.
	DefaultBasket = ""

	// DefaultTags is the default tags to list outputs from.
	DefaultTags = []string{}

	// DefaultTagQueryMode is the default mode for querying tags (All or Any).
	DefaultTagQueryMode = sdk.QueryModeAny
)

// defaultListOutputsArgs creates default arguments for listing wallet outputs.
func defaultListOutputsArgs() sdk.ListOutputsArgs {
	return sdk.ListOutputsArgs{
		Basket:        DefaultBasket,         // Empty basket means list from all baskets
		Tags:          DefaultTags,           // Empty tags means list all outputs regardless of tags
		TagQueryMode:  DefaultTagQueryMode,   // How to query multiple tags (Any/All)
		Limit:         &DefaultLimit,         // Maximum number of outputs to return (100)
		Offset:        &DefaultOffset,        // Starting position for pagination (0 = start from beginning)
		IncludeLabels: &DefaultIncludeLabels, // Include labels associated with outputs in the response
	}
}

// This example demonstrates how to list outputs for the Alice wallet using default arguments.
// It shows the complete flow from wallet creation to output listing with proper error handling.
func main() {
	show.ProcessStart("List Outputs")
	ctx := context.Background()

	// Create Alice's wallet instance
	alice := example_setup.CreateAlice()

	// Create the wallet interface and establish database connection
	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	show.Step("Alice", "Listing outputs")

	// Configure pagination and filtering parameters
	args := defaultListOutputsArgs()
	show.Info("ListOutputsArgs", args)
	show.Separator()

	// Retrieve paginated list of wallet outputs
	outputs, err := aliceWallet.ListOutputs(ctx, args, DefaultOriginator)
	if err != nil {
		panic(fmt.Errorf("failed to list outputs: %w", err))
	}

	show.Info("Outputs", outputs)
	show.ProcessComplete("List Outputs")
}
