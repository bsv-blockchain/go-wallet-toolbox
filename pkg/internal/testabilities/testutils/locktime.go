package testutils

import (
	"context"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/transaction"
)

const MaxSeq = 0xFFFFFFFF

// NewTestTransactionWithLocktime builds a minimal tx with provided locktime and sequences.
func NewTestTransactionWithLocktime(t testing.TB, lock uint32, inputSequences ...uint32) *sdk.Transaction {
	t.Helper()
	tx := sdk.NewTransaction()
	tx.LockTime = lock
	tx.Inputs = make([]*sdk.TransactionInput, len(inputSequences))
	for i, s := range inputSequences {
		tx.Inputs[i] = &sdk.TransactionInput{SequenceNumber: s}
	}
	return tx
}

type StubHeight struct {
	h   uint32
	err error
}

func (s StubHeight) CurrentHeight(context.Context) (uint32, error) { return s.h, s.err }

func NewStubHeight(h uint32, err error) StubHeight {
	return StubHeight{h: h, err: err}
}
