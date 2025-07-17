package example_setup

import (
	"context"
	"encoding/hex"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/utils"
)

// InternalizeFromFaucet is a helper function to internalize a transaction from the faucet
func InternalizeFromFaucet(ctx context.Context, beefHex string, wallet sdk.Interface) error {

	beef, err := hex.DecodeString(beefHex)
	if err != nil {
		show.Error(err.Error())
		return err
	}

	paymentRemittance := utils.DerivationParts()

	senderIdentityKey, err := ec.PublicKeyFromString(paymentRemittance.SenderIdentityKey)
	if err != nil {
		return err
	}

	internalizeArgs := sdk.InternalizeActionArgs{
		Tx: beef,
		Outputs: []sdk.InternalizeOutput{
			{
				OutputIndex: 0,
				Protocol:    "wallet payment",
				PaymentRemittance: &sdk.Payment{
					DerivationPrefix:  paymentRemittance.DerivationPrefix,
					DerivationSuffix:  paymentRemittance.DerivationSuffix,
					SenderIdentityKey: senderIdentityKey,
				},
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
