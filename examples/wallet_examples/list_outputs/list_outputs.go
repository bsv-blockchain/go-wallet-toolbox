package main

import (
	"context"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

const (
	// DefaultLimit is the default number of outputs to retrieve
	DefaultLimit = 100
	// DefaultOffset is the default starting position for pagination
	DefaultOffset = 0
	// DefaultTagQueryMode specifies how to match tags 
	DefaultTagQueryMode = "any"
	// DefaultOriginatorContext is the context for listing outputs
	DefaultOriginatorContext = "originator"
)

// newDefaultListOutputsArgs creates a ListOutputsArgs struct with sensible defaults
// for listing wallet outputs. Returns empty basket and tags to list all outputs.
func newDefaultListOutputsArgs() sdk.ListOutputsArgs {
	return sdk.ListOutputsArgs{
		Basket:       "",                  // Empty basket means list from all baskets
		Tags:         []string{},          // Empty tags means list all outputs regardless of tags
		Limit:        DefaultLimit,        // Maximum number of outputs to return
		Offset:       DefaultOffset,       // Starting position for pagination
		TagQueryMode: DefaultTagQueryMode, // How to match tags when provided
	}
}

// This example demonstrates how to list outputs for the Alice wallet using default arguments
// It shows the complete flow from wallet creation to output listing with proper error handling
func main() {
	show.ProcessStart("List Outputs")

	ctx := context.Background()

	alice := example_setup.CreateAlice()

	aliceWallet, cleanup, err := alice.CreateWallet(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to create Alice's wallet: %w", err))
	}
	defer cleanup()

	show.Step("Alice", "Listing outputs")
	args := newDefaultListOutputsArgs()

	outputs, err := aliceWallet.ListOutputs(ctx, args, DefaultOriginatorContext)
	if err != nil {
		panic(fmt.Errorf("failed to list outputs: %w", err))
	}

	show.Info("Outputs", outputs)

	show.ProcessComplete("List Outputs")
}
