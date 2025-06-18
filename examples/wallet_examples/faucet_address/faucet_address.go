package main

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/examples/example_setup"
)

func main() {
	alice := example_setup.CreateAlice()

	address := example_setup.FaucetAddress(alice)

	fmt.Printf("Generated faucet address: %s\n", address)
}
