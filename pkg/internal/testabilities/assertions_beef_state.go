package testabilities

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// ExpectedBeefTransactionState describes the expected state of a single transaction inside a BEEF.
//
// BumpIndex is optional. When set, the assertion verifies that the tx's BumpIndex field in the
// serialized (round-tripped) BEEF equals the given value. This catches bugs where the in-memory
// BEEF looks correct but the serialized bytes encode a wrong (e.g. zero) bump index.
type ExpectedBeefTransactionState struct {
	ID         string
	DataFormat *transaction.DataFormat
	BumpIndex  *int
}

type beefConstructor func() (*transaction.Beef, error)

func assertBEEFState(t *testing.T, constructor beefConstructor, expectedTxs ...ExpectedBeefTransactionState) {
	beef, err := constructor()
	require.NoError(t, err)
	require.NotNil(t, beef)

	for _, expectedTx := range expectedTxs {
		hash, err := chainhash.NewHashFromHex(expectedTx.ID)
		require.NoError(t, err)
		require.NotNil(t, hash)

		actualTx, ok := beef.Transactions[to.Value(hash)]
		require.Truef(t, ok, "tx with known tx id: %s was expected to be a part of BEEF Transactions tree", expectedTx.ID)

		if expectedTx.DataFormat != nil {
			assert.Equal(t, to.Value(expectedTx.DataFormat), actualTx.DataFormat)
		}

		if expectedTx.BumpIndex != nil {
			assert.Equal(t, to.Value(expectedTx.BumpIndex), actualTx.BumpIndex,
				"BumpIndex for tx %s must match the expected value in the serialized BEEF; "+
					"a mismatch here means the bump index was clobbered during BEEF assembly "+
					"and the recipient would attach the wrong merkle proof", expectedTx.ID)
		}
	}
}

// AssertAtomicBEEFState deserializes atomicBEEF via NewBeefFromAtomicBytes and asserts each
// expected transaction is present. Because it operates on round-tripped bytes it also catches
// wire-level bugs (e.g. BumpIndex encoded as 0) that are invisible when inspecting the
// in-memory *Beef directly.
func AssertAtomicBEEFState(t *testing.T, atomicBEEF []byte, expectedTxs ...ExpectedBeefTransactionState) {
	assertBEEFState(t, func() (*transaction.Beef, error) {
		beef, _, err := transaction.NewBeefFromAtomicBytes(atomicBEEF)
		return beef, err
	}, expectedTxs...)
}

// AssertBEEFState deserializes inputBEEF via NewBeefFromBytes and asserts each expected
// transaction is present.
func AssertBEEFState(t *testing.T, inputBEEF primitives.ExplicitByteArray, expectedTxs ...ExpectedBeefTransactionState) {
	assertBEEFState(t, func() (*transaction.Beef, error) {
		return transaction.NewBeefFromBytes(inputBEEF)
	}, expectedTxs...)
}
