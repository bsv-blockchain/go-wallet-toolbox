package methods

import (
	"context"
	"fmt"
	"log"

	"github.com/4chain-ag/go-wallet-toolbox/examples/src/constants"
	"github.com/4chain-ag/go-wallet-toolbox/examples/src/core"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet"
)

type createWalletsResult struct {
	Wallet1 *wallet.Wallet
	Wallet2 *wallet.Wallet
}

func createWallets(ctx context.Context, network defs.BSVNetwork) (createWalletsResult, error) {
	env := constants.GetEnv(network)

	wallet1, err := core.GetWallet(ctx, network, env.DevKeys.PrivateKey)
	if err != nil {
		return createWalletsResult{}, err
	}

	wallet2, err := core.GetWallet(ctx, network, env.DevKeys.PrivateKey2)
	if err != nil {
		return createWalletsResult{}, err
	}

	return createWalletsResult{
		Wallet1: wallet1,
		Wallet2: wallet2,
	}, nil
}

func CreateWalletsHandler() createWalletsResult {
	ctx := context.Background()
	network := defs.NetworkTestnet

	res, err := createWallets(ctx, network)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("Done")
	fmt.Println("Wallet1", res.Wallet1)
	fmt.Println("Wallet2", res.Wallet2)

	return res
}
