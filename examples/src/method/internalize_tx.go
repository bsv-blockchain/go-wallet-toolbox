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

func InternalizeFromFaucet(txID string, wallet *sdk.Wallet) error {
	if txID == "" {
		panic("")
	}

	// 1 - get the beef from whats on chain /beef endpoint - see wocAPIGetBeefForTX and implementation here: https://github.com/4chain-ag/wallet-toolbox-examples/blob/55d8e75ce7cbf098b8b2b1e4a79fa7d47ba3d0fc/src/act/faucetInternalize.ts#L19
	// - if internal server error is returned then we need probably to wait until tx is mined
	//   * maybe we could send the feedback to Deggen that there is such an issue with the /beef endpoint
	// - later we can use storage method for getting beef
	// 2. When we have beef, prepare Internalize Action Args based on values from DerivationParts
	// 3. Print arguments
	// 4. Call wallet.InternalizeAction
	// 5. Print result

}
