package methods

import (
	"context"
	"fmt"
	"log"

	"github.com/4chain-ag/go-wallet-toolbox/examples/src/core"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
)

type getAddressesResult struct {
	Address1 string
	Address2 string
	IdentityKey1 string
	IdentityKey2 string
}

func getAddresses(ctx context.Context, network defs.BSVNetwork) (getAddressesResult error) {

	res := CreateWalletsHandler()

	parts := core.DerivationParts()


	fmt.Printf("Wallet1: %v\n", res.Wallet1)
	fmt.Printf("Wallet2: %v\n", res.Wallet2)
	fmt.Printf("Parts: %v\n", parts)

	//TODO: Need to work out how to get the address from derivation values using the wallet interface

	// address1 := wallet1.GetPublicKey()
	// address2 := wallet2.KeyDerivation().GetAddress(parts)

	// return getAddressesResult{
	// 	Address1: address1,
	// 	Address2: address2,
	// 	IdentityKey1: env.DevKeys.PrivateKey,
	// 	IdentityKey2: env.DevKeys.PrivateKey2,
	// }

	return nil
}

func GetAddressesHandler() {
	ctx := context.Background()
	network := defs.NetworkTestnet

	if err := getAddresses(ctx, network); err != nil {
		log.Fatalf("Error: %v", err)
	}

	//TODO: we will print the addresses and identity keys here
	fmt.Println("Done")
}
