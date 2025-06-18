package main

import (
	"context"
	"fmt"
	"os"

	"github.com/4chain-ag/go-wallet-toolbox/examples/example_setup"
	"github.com/spf13/pflag"
)

// const beef = ""

var txID = pflag.String("txid", "15f47f2db5f26469c081e8d80d91a4b0f06e4a97abcc022b0b5163ac5f6cc0c8", "pass the chosen txid or simply change the default value when running the example")

func main() {
	ctx := context.Background()

	pflag.Parse()

	//TODO: implement this and create utility function to handle the console messages
	// show.Step(Alice, "")
	// show.Notice("After create action bob doesn't know about transaction")

	alice := example_setup.CreateAlice()

	//TODO:
	// Maybe instead we should have alice.GetWallet() which will have lazy initialization of the wallet 🤷
	aliceWallet, cleanup, err := alice.CreateWallet(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to create Alice's wallet: %w", err))
	}
	defer cleanup()

	// Maybe allow for beef from const also
	// if beef == "" {
	//    beef = methods.GetBEEFHex(txID)
	// }
	// TODO: look at the method for InternalizeFromFaucet the beef as optional param for
	// InternalizeFromFaucet
	//

	err = example_setup.InternalizeFromFaucet(ctx, alice.Environment, *txID, aliceWallet)
	if err != nil {
		fmt.Println("Failed to internalize tx")
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("Transaction %s internalized successfully", *txID)

}
