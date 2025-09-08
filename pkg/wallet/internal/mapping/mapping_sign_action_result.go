package mapping

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

func MapSignActionResultFromStorageResultsForNewTx(txID *chainhash.Hash, tx *assembler.AssembledTransaction, processActionResult *wdk.ProcessActionResult, wdkArgs wdk.ValidCreateActionArgs) (*wallet.SignActionResult, error) {
	sendWithResults, err := MapSendWithResultsFromWDKToSDK(processActionResult.SendWithResults)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare SendWithResults: %w", err)
	}

	result := &wallet.SignActionResult{
		Txid:            to.Value(txID),
		Tx:              nil,
		SendWithResults: sendWithResults,
	}

	if !wdkArgs.Options.ReturnTXIDOnly.Value() {
		result.Tx, err = tx.AtomicBEEF(true)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare atomic beef from result transaction: %w", err)
		}
	}
	return result, nil
}
