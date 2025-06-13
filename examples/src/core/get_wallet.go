package core

import (
	"context"

	"github.com/4chain-ag/go-wallet-toolbox/examples/src/constants"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet"
)

func GetWallet(ctx context.Context, network defs.BSVNetwork, rootKeyHex string) (*wallet.Wallet, error) {
	env := constants.GetEnv(network)

	wallet, cleanup, err := CreateWallet(ctx, env, rootKeyHex)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	return wallet, nil
}
