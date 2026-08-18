package txutils_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
)

// Hydration relies on a go-sdk invariant that its BEEF reader once broke: a
// TxIDOnly entry carries no transaction, and an input whose source is one is
// left unlinked. While the reader attached an empty placeholder instead, every
// bare txid on the wire produced a raw child wired to a parent with no outputs -
// which hydration then kept in preference to the real parent, and which panicked
// the script verifier that consumes the result (go-sdk#345).
//
// This pins the contract the toolbox depends on, so a pin that regressed it
// fails here rather than somewhere deep in createAction.
func TestHydrateBEEFHandlesABareTxIDReadBackFromTheWire(t *testing.T) {
	parentSpec := txtestabilities.GivenTX().WithInput(200_000).WithP2PKHOutput(150_000)
	childSpec := txtestabilities.GivenTX().
		WithSender(txtestabilities.Bob).
		WithInputFromUTXO(parentSpec.TX(), 0).
		WithP2PKHOutput(100_000)

	built := transaction.NewBeefV2()
	_, err := built.MergeRawTx(childSpec.TX().Bytes(), nil)
	require.NoError(t, err)
	built.MergeTxidOnly(parentSpec.ID())

	raw, err := built.Bytes()
	require.NoError(t, err)

	beef, err := transaction.NewBeefFromBytes(raw)
	require.NoError(t, err)

	parentEntry := beef.Transactions[*parentSpec.ID()]
	require.NotNil(t, parentEntry)
	require.Equal(t, transaction.TxIDOnly, parentEntry.DataFormat)
	assert.Nil(t, parentEntry.Transaction, "go-sdk must not attach a transaction to a bare txid")

	childEntry := beef.Transactions[*childSpec.ID()]
	require.NotNil(t, childEntry)
	require.Len(t, childEntry.Transaction.Inputs, 1)
	assert.Nil(t, childEntry.Transaction.Inputs[0].SourceTransaction,
		"go-sdk must not link an input to a source it holds only as a bare txid")

	// And hydration itself must survive the shape: there is nothing to hydrate
	// from a bare txid, so it leaves the input alone rather than failing or
	// dereferencing a missing transaction.
	require.NotPanics(t, func() {
		require.NoError(t, txutils.HydrateBEEF(beef))
	})

	assert.Nil(t, childEntry.Transaction.Inputs[0].SourceTransaction)
}
