package example_setup

import (
	"context"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/examples/utils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func InternalizeTxHandler(ctx context.Context, internalizeArgs sdk.InternalizeActionArgs, wallet sdk.Interface) (sdk.InternalizeActionResult, error) {

	iar, err := wallet.InternalizeAction(ctx, internalizeArgs, "originator")
	if err != nil {
		return sdk.InternalizeActionResult{}, err
	}

	return *iar, nil
}

func InternalizeFromFaucet(ctx context.Context, env Environment, txID string, wallet sdk.Interface) error {
	if txID == "" {
		panic("txID cannot be empty")
	}

	//TODO: fix the issue with the beef format error -  failed to create transaction from BEEF: use NewBeefFromBytes to parse anything which isn't V1 BEEF or AtomicBEEF
	beef, err := utils.WocAPIGetBeefForTX(string(defs.NetworkTestnet), txID)
	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Println("Beef: ", beef)

	paymentRemittance := utils.DerivationParts().PaymentRemittance

	internalizeArgs := sdk.InternalizeActionArgs{
		Tx: []byte(beef),
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
	// 1 - get the beef from whats on chain /beef endpoint - see wocAPIGetBeefForTX and implementation here: https://github.com/4chain-ag/wallet-toolbox-examples/blob/55d8e75ce7cbf098b8b2b1e4a79fa7d47ba3d0fc/src/act/faucetInternalize.ts#L19
	// - if internal server error is returned then we need probably to wait until tx is mined
	//   * maybe we could send the feedback to Deggen that there is such an issue with the /beef endpoint
	// - later we can use storage method for getting beef
	// 2. When we have beef, prepare Internalize Action Args based on values from DerivationParts
	// 3. Print arguments
	// 4. Call wallet.InternalizeAction
	// 5. Print result

}
