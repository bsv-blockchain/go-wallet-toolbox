package sdkbeef_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/sdkbeef"
)

// A round trip through the BEEF wire format turns a bare txid into a bare txid
// PLUS an empty transaction object, which go-sdk then wires into whatever spends
// it. This pins both the defect and the repair: without ParseBytes the child's
// source is the empty placeholder, and touching it panics.
func TestParseBytesDropsTheEmptyTransactionLeftBehindByABareTxID(t *testing.T) {
	parentSpec := txtestabilities.GivenTX().WithInput(200_000).WithP2PKHOutput(150_000)
	childSpec := txtestabilities.GivenTX().
		WithSender(txtestabilities.Bob).
		WithInputFromUTXO(parentSpec.TX(), 0).
		WithP2PKHOutput(100_000)

	beef := transaction.NewBeefV2()
	_, err := beef.MergeRawTx(childSpec.TX().Bytes(), nil)
	require.NoError(t, err)
	beef.MergeTxidOnly(parentSpec.ID())

	raw, err := beef.Bytes()
	require.NoError(t, err)

	// the defect, as go-sdk returns it:
	unsanitized, err := transaction.NewBeefFromBytes(raw)
	require.NoError(t, err)

	parentEntry := unsanitized.Transactions[*parentSpec.ID()]
	require.NotNil(t, parentEntry)
	require.Equal(t, transaction.TxIDOnly, parentEntry.DataFormat)
	require.NotNil(t, parentEntry.Transaction, "go-sdk attaches an empty transaction to a bare txid")
	require.Empty(t, parentEntry.Transaction.Outputs)

	childEntry := unsanitized.Transactions[*childSpec.ID()]
	require.NotNil(t, childEntry)
	require.NotNil(t, childEntry.Transaction.Inputs[0].SourceTransaction,
		"go-sdk wires the empty transaction in as the child's source")
	require.Empty(t, childEntry.Transaction.Inputs[0].SourceTransaction.Outputs)

	// and the repair:
	sanitized, err := sdkbeef.ParseBytes(raw)
	require.NoError(t, err)

	sanitizedParent := sanitized.Transactions[*parentSpec.ID()]
	require.NotNil(t, sanitizedParent)
	assert.Equal(t, transaction.TxIDOnly, sanitizedParent.DataFormat)
	assert.Nil(t, sanitizedParent.Transaction, "a bare txid must carry no transaction")
	assert.NotNil(t, sanitizedParent.KnownTxID)

	sanitizedChild := sanitized.Transactions[*childSpec.ID()]
	require.NotNil(t, sanitizedChild)
	assert.Nil(t, sanitizedChild.Transaction.Inputs[0].SourceTransaction,
		"the child must not point at a placeholder it cannot spend")
}

// The empty transaction spreads: merging a graph that already carries one adds it
// as an entry of its own, under the txid of ten zero bytes.
func TestSanitizeRemovesAnEmptyTransactionAlreadyMergedAsAnEntry(t *testing.T) {
	empty := &transaction.Transaction{}
	emptyTxID := empty.TxID()

	beef := transaction.NewBeefV2()
	_, err := beef.MergeTransaction(empty)
	require.NoError(t, err)
	require.Contains(t, beef.Transactions, *emptyTxID)

	sdkbeef.Sanitize(beef)

	assert.NotContains(t, beef.Transactions, *emptyTxID)
}
