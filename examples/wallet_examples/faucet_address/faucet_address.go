package main

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
)

// This method will print the faucet address to the console which can then be used to recieve funds from the testnet faucet
// The funds from the transaction will be used to by the faucet_internalize example to add it to the wallet database
func main() {
	alice := example_setup.CreateAlice()

	example_setup.FaucetAddress(alice)

}
