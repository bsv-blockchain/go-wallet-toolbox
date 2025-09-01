package example_setup

import (
	"context"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/utils"
)

// InternalizeFromFaucet is a helper function to internalize a transaction from the faucet
func InternalizeFromFaucet(ctx context.Context, atomicBeefBytes []byte, wallet sdk.Interface) error {
	paymentRemittance := utils.DerivationParts()

	internalizeArgs := sdk.InternalizeActionArgs{
		Tx: atomicBeefBytes,
		Outputs: []sdk.InternalizeOutput{
			{
				OutputIndex:       0,
				Protocol:          "wallet payment",
				PaymentRemittance: paymentRemittance,
			},
		},
		Description: "internalize from faucet",
	}

	iar, err := wallet.InternalizeAction(ctx, internalizeArgs, "originator")
	if err != nil {
		show.WalletError("InternalizeAction", internalizeArgs, err)
		return err
	}

	show.WalletSuccess("InternalizeAction", internalizeArgs, *iar)
	return nil
}
