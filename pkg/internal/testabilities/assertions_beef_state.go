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

type ExpectedBeefTransactionState struct {
	ID         string
	DataFormat *transaction.DataFormat
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
	}
}

func AssertAtomicBEEFState(t *testing.T, atomicBEEF []byte, expectedTxs ...ExpectedBeefTransactionState) {
	assertBEEFState(t, func() (*transaction.Beef, error) {
		beef, _, err := transaction.NewBeefFromAtomicBytes(atomicBEEF)
		return beef, err
	}, expectedTxs...)
}

func AssertBEEFState(t *testing.T, inputBEEF primitives.ExplicitByteArray, expectedTxs ...ExpectedBeefTransactionState) {
	assertBEEFState(t, func() (*transaction.Beef, error) {
		return transaction.NewBeefFromBytes(inputBEEF)
	}, expectedTxs...)
}
