package testabilities

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ExpectedBeefTransactionState struct {
	ID         string
	DataFormat *transaction.DataFormat
}

func AssertBEEFState(t *testing.T, inputBEEF primitives.ExplicitByteArray, expectedTxs ...ExpectedBeefTransactionState) {
	beef, err := transaction.NewBeefFromBytes(inputBEEF)
	require.NoError(t, err)
	require.NotNil(t, beef)

	for _, expectedTx := range expectedTxs {
		hash, err := chainhash.NewHashFromHex(expectedTx.ID)
		require.NoError(t, err)
		require.NotNil(t, hash)

		actualTx, ok := beef.Transactions[to.Value(hash)]
		require.True(t, ok, "tx with known tx id: %s was exepcted to be a part of BEEF Transactions tree", expectedTx.ID)

		if expectedTx.DataFormat != nil {
			assert.Equal(t, actualTx.DataFormat, to.Value(expectedTx.DataFormat))
		}
	}
}
