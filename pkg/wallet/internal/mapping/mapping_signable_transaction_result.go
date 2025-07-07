package mapping

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/beefutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

func SignableTransactionResult(tx *transaction.Transaction, wdkArgs wdk.ValidCreateActionArgs, storageResult *wdk.StorageCreateActionResult) (*sdk.CreateActionResult, error) {
	txID := tx.TxID()

	atomicBytes, err := beefutils.ToAtomicBEEFBytes(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to create atomic tx bytes: %w", err)
	}

	result := &sdk.CreateActionResult{
		// TODO: make an issue/PR, that it should be pointer
		Txid: to.Value(txID),
		SignableTransaction: &sdk.SignableTransaction{
			// TODO: make an issue/PR, that it should be string
			Reference: []byte(storageResult.Reference),
			Tx:        atomicBytes,
		},
	}

	if wdkArgs.IsNoSend && storageResult.NoSendChangeOutputVouts != nil {
		result.NoSendChange, err = MapIndexesToOutpoints(txID, *storageResult.NoSendChangeOutputVouts)
		if err != nil {
			return nil, fmt.Errorf("failed to build noSendChange result: %w", err)
		}
	}

	return result, nil
}
