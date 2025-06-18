package main

import (
	"context"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/examples/example_setup"
	"github.com/4chain-ag/go-wallet-toolbox/examples/show"
	"github.com/4chain-ag/go-wallet-toolbox/examples/utils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/spf13/pflag"
)

var beef = ""

var txID = pflag.String("txid", "15f47f2db5f26469c081e8d80d91a4b0f06e4a97abcc022b0b5163ac5f6cc0c8", "pass the chosen txid or simply change the default value when running the example")

func main() {
	show.ProcessStart("Faucet Transaction Internalization")

	ctx := context.Background()
	pflag.Parse()

	show.Step("Alice", "Creating wallet and setting up environment")

	alice := example_setup.CreateAlice()

	aliceWallet, cleanup, err := alice.CreateWallet(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to create Alice's wallet: %w", err))
	}
	defer cleanup()

	if beef == "" {
		beef, err = utils.WocAPIGetBeefForTX(string(defs.NetworkTestnet), *txID)
		if err != nil {
			panic(fmt.Errorf("failed to get beef for tx: %w", err))
		}

		show.Step("Alice", "Retrieving BEEF data for transaction")
		show.Transaction(*txID)

	}

	show.Step("Alice", "Internalizing transaction from faucet")

	err = example_setup.InternalizeFromFaucet(ctx, alice.Environment, beef, aliceWallet)
	if err != nil {
		panic(fmt.Errorf("failed to internalize tx: %w", err))
	}

	show.Success("Transaction internalized successfully")
	show.ProcessComplete("Faucet Transaction Internalization")

}
