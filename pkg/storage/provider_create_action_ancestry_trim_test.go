package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	pkgtestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
)

// A sign action receives every allocated input's source transaction inline, in
// StorageCreateTransactionSdkInput.SourceTransaction, filled by one lookup per
// input. The assembler prefers that field over the response BEEF when it builds
// the signable transaction, so walking the ancestry of those inputs produces
// data nothing reads - and on a deep chain that walk is the dominant cost of
// createAction. It collapses to TxIDOnly stubs instead.
//
// See CreateActionParams.ancestryIsRedundant.

func TestCreateAction_SignAction_TrimsAllocatedInputAncestry(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// given: a funded UTXO the funder will allocate
	faucetTx, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and: a sign action - IncludeAllSourceTransactions defaults to true, so
	// storage returns each input's source transaction inline
	args := fixtures.DefaultValidCreateActionArgs()
	args.IsSignAction = true
	require.True(t, args.IncludeAllSourceTransactions, "precondition: source txs are returned inline")

	// when:
	result, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), args)

	// then: the allocated input's ancestry is a stub, not a full transaction
	require.NoError(t, err)
	pkgtestabilities.AssertBEEFState(t, result.InputBeef, pkgtestabilities.ExpectedBeefTransactionState{
		ID:         faucetTx.ID().String(),
		DataFormat: to.Ptr(transaction.TxIDOnly),
	})

	// and: signing still has everything it needs, from the input itself
	require.NotEmpty(t, result.Inputs)
	for _, input := range result.Inputs {
		assert.NotEmpty(t, input.SourceTransaction,
			"a trimmed ancestry is only safe because the source transaction is returned inline")
		assert.NotEmpty(t, input.SourceLockingScript)
	}
}

// The contrast case: a caller that is NOT a sign action consumes the response
// BEEF itself (cross-wallet handoff, beef party learning), so the ancestry has
// to arrive in full.
func TestCreateAction_NonSignAction_KeepsAllocatedInputAncestry(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	faucetTx, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	args := fixtures.DefaultValidCreateActionArgs()
	require.False(t, args.IsSignAction, "precondition: not a sign action")

	// when:
	result, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), args)

	// then: the full transaction is present, not a stub
	require.NoError(t, err)
	pkgtestabilities.AssertBEEFState(t, result.InputBeef, pkgtestabilities.ExpectedBeefTransactionState{
		ID:         faucetTx.ID().String(),
		DataFormat: to.Ptr(transaction.RawTx),
	})
}
