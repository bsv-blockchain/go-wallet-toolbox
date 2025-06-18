package example_setup

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/examples/utils"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func InternalizeTxHandler(ctx context.Context, internalizeArgs sdk.InternalizeActionArgs, wallet sdk.Interface) (sdk.InternalizeActionResult, error) {

	iar, err := wallet.InternalizeAction(ctx, internalizeArgs, "originator")
	if err != nil {
		return sdk.InternalizeActionResult{}, err
	}

	return *iar, nil
}

func InternalizeFromFaucet(ctx context.Context, env Environment, beefHex string, wallet sdk.Interface) error {

	beef, err := hex.DecodeString(beefHex)
	if err != nil {
		fmt.Println(err)
		return err
	}

	paymentRemittance := utils.DerivationParts().PaymentRemittance

	internalizeArgs := sdk.InternalizeActionArgs{
		Tx: beef,
		Outputs: []sdk.InternalizeOutput{
			{
				OutputIndex:       0,
				Protocol:          "wallet payment",
				PaymentRemittance: paymentRemittance,
			},
		},
		Description: "internalize from faucet",
	}

	fmt.Println(internalizeArgs)

	internalizeResult, err := InternalizeTxHandler(ctx, internalizeArgs, wallet)
	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Println(internalizeResult)

	return nil

}
