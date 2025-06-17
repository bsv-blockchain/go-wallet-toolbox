package core

import (
	"context"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/examples/src/constants"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet"
)

// GetWallet gets an existing wallet from the database
func GetWallet(ctx context.Context, network defs.BSVNetwork, rootKeyHex string) (*wallet.Wallet, func(), error) {
	env := constants.GetEnv(network)

	storageClient, cleanup, err := storage.NewClient(env.ServerURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	wallet, err := wallet.New(env.Network, rootKeyHex, storageClient)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to create wallet: %w", err)
	}

	return wallet, cleanup, nil
}
