package mapping

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

func MapCreateActionResultFromStorageResultsForNewTx(txID *chainhash.Hash, tx *assembler.AssembledTransaction, createActionResult *wdk.StorageCreateActionResult, processActionResult *wdk.ProcessActionResult, wdkArgs wdk.ValidCreateActionArgs) (*wallet.CreateActionResult, error) {
	noSendChange, err := MapIndexesToOutpoints(txID, createActionResult.NoSendChangeOutputVouts)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare no send change outpoints, %w", err)
	}

	sendWithResults, err := MapSendWithResultsFromWDKToSDK(processActionResult.SendWithResults)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare SendWithResults: %w", err)
	}

	result := &wallet.CreateActionResult{
		Txid:            to.Value(txID),
		NoSendChange:    noSendChange,
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

func MapCreateActionResultFromStorageResultsForSendWith(processActionResult *wdk.ProcessActionResult) (*wallet.CreateActionResult, error) {
	sendWithResults, err := MapSendWithResultsFromWDKToSDK(processActionResult.SendWithResults)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare SendWithResults: %w", err)
	}

	result := &wallet.CreateActionResult{
		SendWithResults: sendWithResults,
	}

	return result, nil
}
