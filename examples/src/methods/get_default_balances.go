package methods

import (
	"context"
	//"fmt"
	"math/big"

	// "github.com/4chain-ag/go-wallet-toolbox/examples/src/constants"
	// "github.com/4chain-ag/go-wallet-toolbox/examples/src/core"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
)

type getDefaultBalancesResult struct {	
	Balance1 *big.Int
	Balance2 *big.Int
}

func GetDefaultBalances(ctx context.Context, network defs.BSVNetwork) (getDefaultBalancesResult, error) {
	//TODO: We will assume there is no default balance method for the wallet.
	// We will use the wallet interface to get the outputs 
	// We will use the utils method for accumulation.
	// Template code provided below
	panic("implement me")

	// env := constants.GetEnv(network)

	// wallet1, err := core.GetWallet(ctx, network, env.DevKeys.PrivateKey)
	// if err != nil {
	// 	return nil, nil, err
	// }

	// wallet2, err := core.GetWallet(ctx, network, env.DevKeys.PrivateKey2)
	// if err != nil {
	// 	return nil, nil, err
	// }

	// balance1, err := wallet1.Balance(ctx)
	// if err != nil {
	// 	return nil, nil, err
	// }

	// balance2, err := wallet2.Balance(ctx)
	// if err != nil {
	// 	return nil, nil, err
	// }

	// fmt.Printf("Wallet 1 balances: %v\n", balance1)
	// fmt.Printf("Wallet 2 balances: %v\n", balance2)
	
	// return balance1, balance2, nil
}
