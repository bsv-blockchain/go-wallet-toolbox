package main

import (
	"github.com/4chain-ag/go-wallet-toolbox/examples/example_setup"
)


func main() {
	alice := example_setup.CreateAlice()

	// This method will print the faucet address to the console which can then be used to recieve funds from the testnet faucet
	// The funds from the transaction will be used to by the faucet_internalize example to add it to the wallet database
	example_setup.FaucetAddress(alice)

}
