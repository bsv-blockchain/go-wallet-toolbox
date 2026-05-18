package assembler

import (
	"fmt"
	"iter"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/seqerr"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/pending"
)

type AssembledTransaction struct {
	*transaction.Transaction

	inputBEEF   *transaction.Beef
	initialTxID *chainhash.Hash
}

func NewAssembledTxFromPendingSignAction(pendingSignAction *pending.SignAction) *AssembledTransaction {
	return &AssembledTransaction{
		Transaction: &pendingSignAction.Tx,
		inputBEEF:   pendingSignAction.InputBEEF,
		initialTxID: pendingSignAction.Tx.TxID(),
	}
}

func (a *AssembledTransaction) AtomicBEEF(allowPartials bool) ([]byte, error) {
	beef, err := a.ToAtomicBEEF(allowPartials)
	if err != nil {
		return nil, fmt.Errorf("failed to build beef from assembled tx: %w", err)
	}
	bytes, err := beef.AtomicBytes(a.TxID())
	if err != nil {
		return nil, fmt.Errorf("failed to serialize assembled transaction to atomic beef bytes: %w", err)
	}

	return bytes, nil
}

func (a *AssembledTransaction) ToAtomicBEEF(allowPartials bool) (*transaction.Beef, error) {
	beef := transaction.NewBeef()

	err := beef.MergeBeef(a.inputBEEF)
	if err != nil {
		return nil, fmt.Errorf("failed to merge input beef into transaction beef: %w", err)
	}

	if a.initialTxID != nil {
		delete(beef.Transactions, *a.initialTxID)
	}

	allInputs := seqerr.FromSlice(a.Inputs)

	var inputsWithSourceTx iter.Seq2[*transaction.TransactionInput, error]
	if !allowPartials {
		inputsWithSourceTx = seqerr.Filter(allInputs, validateInputs)
	} else {
		inputsWithSourceTx = seqerr.Filter(allInputs, func(it *transaction.TransactionInput) bool {
			return it.SourceTransaction != nil
		})
	}

	inputSourceTxs := seqerr.Map(inputsWithSourceTx, inputSourceTransaction)

	err = seqerr.ForEach(inputSourceTxs, mergeSourceTxIntoBEEF(beef))
	if err != nil {
		return nil, fmt.Errorf("failed to build beef from tx, %w", err)
	}

	_, err = beef.MergeRawTx(a.Bytes(), nil)
	if err != nil {
		return nil, fmt.Errorf("cannot merge new tx into beef: %w", err)
	}

	return beef, nil
}

func validateInputs(input *transaction.TransactionInput) error {
	if input.SourceTransaction == nil {
		return fmt.Errorf("internal: every signable transaction input must have a source transaction")
	}
	return nil
}

func inputSourceTransaction(input *transaction.TransactionInput) *transaction.Transaction {
	return input.SourceTransaction
}

func mergeSourceTxIntoBEEF(beef *transaction.Beef) func(*transaction.Transaction) error {
	return func(tx *transaction.Transaction) error {
		txid := tx.TxID()
		var bumpIndex *int
		if existing, ok := beef.Transactions[*txid]; ok && existing.DataFormat == transaction.RawTxAndBumpIndex {
			idx := existing.BumpIndex
			bumpIndex = &idx
		}
		_, err := beef.MergeRawTx(tx.Bytes(), bumpIndex)
		if err != nil {
			return fmt.Errorf("cannot merge raw tx into beef: %w", err)
		}
		return nil
	}
}
