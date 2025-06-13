package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/examples/src/constants"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func MatchUser(ctx context.Context, network defs.BSVNetwork, identityKey string) (*wallet.Wallet, error) {
	env := constants.GetEnv(network)
	// We have the public keys hardcoded in the constants file but we will use the GetPublicKey method to get the public key
	// for the given identity key for example practicality

	wallet1, err := GetWallet(ctx, network, env.DevKeys.PrivateKey)
	if err != nil {
		return nil, err
	}

	wallet2, err := GetWallet(ctx, network, env.DevKeys.PrivateKey2)
	if err != nil {
		return nil, err
	}

	key1, err := wallet1.GetPublicKey(ctx, sdk.GetPublicKeyArgs{IdentityKey: true}, "")
	if err != nil {
		return nil, err
	}

	key2, err := wallet2.GetPublicKey(ctx, sdk.GetPublicKeyArgs{IdentityKey: true}, "")
	if err != nil {
		return nil, err
	}

	if key1.PublicKey.ToDERHex() == identityKey {
		fmt.Printf("MatchUser: Found wallet 1: %s\n", key1.PublicKey.ToDERHex())
		return wallet1, nil
	}

	if key2.PublicKey.ToDERHex() == identityKey {
		fmt.Printf("MatchUser: Found wallet 2: %s\n", key2.PublicKey.ToDERHex())
		return wallet2, nil
	}

	return nil, errors.New("no matching wallet found for the given identity key")
}
