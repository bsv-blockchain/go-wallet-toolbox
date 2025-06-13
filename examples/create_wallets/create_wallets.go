package main

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/examples/src/methods"
)

func main() {
	// we will just create both wallets provided in the constants file
	res := methods.CreateWalletsHandler()
	fmt.Println("Wallet1", res.Wallet1)
	fmt.Println("Wallet2", res.Wallet2)
}
