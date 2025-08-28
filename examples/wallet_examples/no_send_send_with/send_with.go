package main

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/wallet"
)

func sendWith(ctx context.Context, aliceWallet wallet.Interface, txIDs []chainhash.Hash) {
	_, err := aliceWallet.CreateAction(ctx, wallet.CreateActionArgs{
		Options: &wallet.CreateActionOptions{
			SendWith: txIDs,
		},
		Description: "sendWith",
	}, "")
	if err != nil {
		panic(err)
	}
}
