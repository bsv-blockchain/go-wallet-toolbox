package txutils_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestContainsUtxo(t *testing.T) {
	txid := "9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6"
	hash, err := chainhash.NewHashFromHex(txid)
	require.NoError(t, err)

	tests := []struct {
		name     string
		details  []wdk.UtxoDetail
		outpoint *transaction.Outpoint
		expected bool
	}{
		{
			name: "UTXO found",
			details: []wdk.UtxoDetail{
				{TxID: txid, Index: 1},
				{TxID: "abc", Index: 2},
			},
			outpoint: &transaction.Outpoint{Txid: *hash, Index: 1},
			expected: true,
		},
		{
			name: "UTXO not found",
			details: []wdk.UtxoDetail{
				{TxID: txid, Index: 0},
			},
			outpoint: &transaction.Outpoint{Txid: *hash, Index: 1},
			expected: false,
		},
		{
			name:     "Empty list",
			details:  nil,
			outpoint: &transaction.Outpoint{Txid: *hash, Index: 1},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			// (everything is already given in test struct)

			// when:
			actual := txutils.ContainsUtxo(tc.details, tc.outpoint)

			// then:
			assert.Equal(t, tc.expected, actual)
		})
	}
}
