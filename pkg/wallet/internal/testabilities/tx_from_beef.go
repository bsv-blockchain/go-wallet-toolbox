package testabilities

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/require"
)

func CreateTxFromBEEF(t *testing.T, beef []byte) *transaction.Transaction {
	t.Helper()
	tx, err := transaction.NewTransactionFromBEEF(beef)
	require.NoError(t, err, "Failed to decode transaction from result")
	require.NotNil(t, tx)
	return tx
}
