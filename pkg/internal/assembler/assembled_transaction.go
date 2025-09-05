package assembler

import (
	"fmt"
	"iter"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/seqerr"
)

type AssembledTransaction struct {
	*transaction.Transaction
	inputBEEF *transaction.Beef

	cachedAtomicBeef      *transaction.Beef
	cachedAtomicBeefBytes []byte
}

func NewAssembledTxFromPendingSignAction(pendingSignAction *wdk.PendingSignAction) *AssembledTransaction {
	return &AssembledTransaction{
		Transaction: &pendingSignAction.Tx,
		inputBEEF:   pendingSignAction.InputBEEF,
	}
}

func (a *AssembledTransaction) AtomicBEEF(allowPartials bool) ([]byte, error) {
	if a.cachedAtomicBeefBytes != nil {
		return a.cachedAtomicBeefBytes, nil
	}
	beef, err := a.ToAtomicBEEF(allowPartials)
	if err != nil {
		return nil, fmt.Errorf("failed to build beef from assembled tx: %w", err)
	}
	bytes, err := beef.AtomicBytes(a.TxID())
	if err != nil {
		return nil, fmt.Errorf("failed to serialize assembled transaction to atomic beef bytes: %w", err)
	}

	a.cachedAtomicBeefBytes = bytes
	return bytes, nil
}

func (a *AssembledTransaction) ToAtomicBEEF(allowPartials bool) (*transaction.Beef, error) {
	if a.cachedAtomicBeef != nil {
		return a.cachedAtomicBeef, nil
	}
	beef := transaction.NewBeef()

	err := beef.MergeBeef(a.inputBEEF)
	if err != nil {
		return nil, fmt.Errorf("failed to merge input beef into transaction beef: %w", err)
	}

	allInputs := seqerr.FromSlice(a.Transaction.Inputs)

	var inputsWithSourceTx iter.Seq2[*transaction.TransactionInput, error]
	if !allowPartials {
		inputsWithSourceTx = seqerr.Filter(allInputs, validateInputs)
	} else {
		inputsWithSourceTx = seqerr.Filter(allInputs, func(it *transaction.TransactionInput) bool {
			return it.SourceTransaction != nil
		})
	}

	inputsRawTx := seqerr.Map(inputsWithSourceTx, inputRawTxBytes)

	allRawTxs := seqerr.Append(inputsRawTx, a.Transaction.Bytes())

	err = seqerr.ForEach(allRawTxs, mergeRawTxIntoBEEF(beef))
	if err != nil {
		return nil, fmt.Errorf("failed to build beef from tx, %w", err)
	}

	a.cachedAtomicBeef = beef
	return beef, nil
}

func validateInputs(input *transaction.TransactionInput) error {
	if input.SourceTransaction == nil {
		return fmt.Errorf("internal: every signable transaction input must have a source transaction")
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
