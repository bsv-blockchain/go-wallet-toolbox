package example_setup

import (
	"context"
	"encoding/hex"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/utils"
)

// InternalizeFromFaucet is a helper function to internalize a transaction from the faucet
func InternalizeFromFaucet(ctx context.Context, beefHex string, wallet sdk.Interface) error {

	beef, err := hex.DecodeString(beefHex)
	if err != nil {
		fmt.Println(err)
		return err
	}

	paymentRemittance := utils.DerivationParts()

	internalizeArgs := sdk.InternalizeActionArgs{
		Tx: beef,
		Outputs: []sdk.InternalizeOutput{
			{
				OutputIndex: 0,
				Protocol:    "wallet payment",
				PaymentRemittance: &sdk.Payment{
					DerivationPrefix:  string(paymentRemittance.DerivationPrefix),
					DerivationSuffix:  string(paymentRemittance.DerivationSuffix),
					SenderIdentityKey: paymentRemittance.SenderIdentityKey,
				},
			},
		},
		Description: "internalize from faucet",
	}

	fmt.Println(internalizeArgs)

	iar, err := wallet.InternalizeAction(ctx, internalizeArgs, "originator")
	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Println(*iar)

	return nil
}
