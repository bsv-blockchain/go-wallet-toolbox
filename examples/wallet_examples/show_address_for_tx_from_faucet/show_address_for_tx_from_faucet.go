package main

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
)

// This method will print the faucet address to the console which can then be used to receive funds from the testnet faucet
// testnet faucets are available at https://scrypt.io/faucet and https://witnessonchain.com/faucet/tbsv
// The funds from the transaction will be used to by the faucet_internalize example to add it to the wallet database
func main() {
	// Create Alice's wallet instance with deterministic keys
	alice := example_setup.CreateAlice()

	// Generate and display a BRC-29 testnet address for receiving faucet funds
	example_setup.FaucetAddress(alice)
}
