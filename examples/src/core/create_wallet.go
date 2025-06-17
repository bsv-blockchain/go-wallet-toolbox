package core

import (
	"context"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/examples/src/constants"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
)

// CreateWallet creates a new wallet and returns it
func CreateWallet(ctx context.Context, env constants.Environment, rootKeyHex string) (*wallet.Wallet, func(), error) {
	privateKey, err := ec.PrivateKeyFromHex(rootKeyHex)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	storageClient, cleanup, err := storage.NewClient(env.ServerURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	userWallet, err := wallet.New(env.Network, privateKey, storageClient)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to create wallet: %w", err)
	}

	identityKey := privateKey.PubKey().ToDERHex()
	user, err := storageClient.FindOrInsertUser(ctx, identityKey)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to find or insert user: %w", err)
	}

	fmt.Printf("CreateWallet: User %d: %s\n", user.User.UserID, identityKey)
	return userWallet, cleanup, nil
}
