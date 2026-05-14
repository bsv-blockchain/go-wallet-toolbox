package assembler_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/require"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/pending"
)

func TestToAtomicBEEF_RemovesUnsignedTx(t *testing.T) {
	// Create a transaction
	tx := &transaction.Transaction{Version: 1}
	unsignedID := tx.TxID()

	// Create a BEEF and add the unsigned transaction to it
	beef := transaction.NewBeef()
	_, err := beef.MergeRawTx(tx.Bytes(), nil)
	require.NoError(t, err)
	require.Contains(t, beef.Transactions, *unsignedID)

	// Create AssembledTransaction from pending action
	pAction := &pending.SignAction{
		Tx:        *tx,
		InputBEEF: beef,
	}
	assembled := assembler.NewAssembledTxFromPendingSignAction(pAction)

	// Now "sign" the transaction (simulated by changing something that changes TXID)
	assembled.Transaction.LockTime = 12345
	signedID := assembled.Transaction.TxID()
	require.NotEqual(t, unsignedID, signedID)

	// Generate BEEF
	finalBeef, err := assembled.ToAtomicBEEF(true)
	require.NoError(t, err)

	// Final BEEF should not contain the unsigned TXID
	require.Contains(t, finalBeef.Transactions, *signedID)
	require.NotContains(t, finalBeef.Transactions, *unsignedID, "Final BEEF should NOT contain the unsigned transaction ID")
}
