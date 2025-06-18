package main

import (
	"github.com/4chain-ag/go-wallet-toolbox/examples/example_setup"
)

func main() {
	alice := example_setup.CreateAlice()

	example_setup.FaucetAddress(alice)

}
