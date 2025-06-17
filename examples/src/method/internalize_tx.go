package methods

import (
	"context"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func InternalizeTxHandler(ctx context.Context, internalizeArgs sdk.InternalizeActionArgs, userWallet sdk.Interface) (sdk.InternalizeActionResult, error) {

	iar, err := userWallet.InternalizeAction(ctx, internalizeArgs, "originator")
	if err != nil {
		return sdk.InternalizeActionResult{}, err
	}

	return *iar, nil
}
