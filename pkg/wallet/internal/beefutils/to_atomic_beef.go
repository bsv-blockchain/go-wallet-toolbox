package beefutils

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/seqerr"
)

func ToAtomicBEEFBytes(tx *transaction.Transaction) ([]byte, error) {
	beef := transaction.NewBeef()

	inputs := seqerr.Filter(seqerr.FromSlice(tx.Inputs), validateInputs)

	inputsRawTx := seqerr.Map(inputs, inputRawTxBytes)

	allRawTxs := seqerr.Append(inputsRawTx, tx.Bytes())

	err := seqerr.ForEach(allRawTxs, mergeRawTxIntoBEEF(beef))
	if err != nil {
		return nil, fmt.Errorf("failed to build beef from tx, %w", err)
	}

	atomicBytes, err := beef.AtomicBytes(tx.TxID())
	if err != nil {
		return nil, fmt.Errorf("failed to create atomic tx bytes: %w", err)
	}

	return atomicBytes, nil
}

func validateInputs(input *transaction.TransactionInput) error {
	if input.SourceTransaction == nil {
		return fmt.Errorf("internal: every signlable transaction input must have a source transaction")
	}
	return nil
}

func inputRawTxBytes(input *transaction.TransactionInput) []byte {
	return input.SourceTransaction.Bytes()
}

func mergeRawTxIntoBEEF(beef *transaction.Beef) func([]byte) error {
	return func(rawTx []byte) error {
		_, err := beef.MergeRawTx(rawTx, nil)
		if err != nil {
			return fmt.Errorf("cannot merge raw tx into beef: %w", err)
		}
		return nil
	}
}
