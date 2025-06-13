package methods

import (
	"context"
	"log"

	"github.com/4chain-ag/go-wallet-toolbox/examples/src/core"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func internalizeTx(ctx context.Context, network defs.BSVNetwork, internalizeArgs sdk.InternalizeActionArgs, identityKey string) (sdk.InternalizeActionResult, error) {

	wallet, err := core.MatchUser(ctx, network, identityKey)
	if err != nil {
		return sdk.InternalizeActionResult{}, err
	}

	iar, err := wallet.InternalizeAction(ctx, internalizeArgs, "originator")
	if err != nil {
		return sdk.InternalizeActionResult{}, err
	}

	return *iar, nil
}

func InternalizeTxHandler(internalizeArgs sdk.InternalizeActionArgs, identityKey string) sdk.InternalizeActionResult {
	ctx := context.Background()
	network := defs.NetworkTestnet

	iar, err := internalizeTx(ctx, network, internalizeArgs, identityKey)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	return iar
}
