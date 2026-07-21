package storage_test

import (
	"testing"

	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/specops"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestUnFail_WithMerklePath_MovesToUnminedAndUnproven(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	provider := given.Provider()
	activeStorage := provider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: create outgoing tx and make it fail (double spend)
	createActionResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()
	otherTXID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
	provider.ARC().WhenQueryingTx(txID).WillReturnDoubleSpending(otherTXID)

	args := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   []primitives.TXIDHexString{},
	}
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)
	require.NoError(t, err)

	// and: mark it for unfail
	_, _ = activeStorage.ListActions(t.Context(), testusers.Alice.AuthID(), wdk.ListActionsArgs{
		Labels: []primitives.StringUnder300{primitives.StringUnder300(wdk.TxStatusUnfail), primitives.StringUnder300(specops.ListActionsSpecOpFailedActionsLabel)},
		Limit:  10,
		Offset: 0,
	})

	// and: mock MerklePath present
	mp := testutils.MockValidMerklePath(t, txID, 2000)
	provider.ARC().WhenQueryingTx(txID).WillReturnTransactionWithMerklePath(mp)

	// when:
	err = activeStorage.UnFail(t.Context())

	// then:
	require.NoError(t, err)
	thenDB := testabilities.ThenDBState(t, activeStorage)
	thenDB.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
	thenDB.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusUnproven)
}

func TestUnFail_NoMerklePath_SetsKnownTxInvalid(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	provider := given.Provider()
	activeStorage := provider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: create outgoing tx and make it fail (double spend)
	createActionResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()
	otherTXID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
	provider.ARC().WhenQueryingTx(txID).WillReturnDoubleSpending(otherTXID)

	args := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   []primitives.TXIDHexString{},
	}
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)
	require.NoError(t, err)

	// and: mark it for unfail
	_, _ = activeStorage.ListActions(t.Context(), testusers.Alice.AuthID(), wdk.ListActionsArgs{
		Labels: []primitives.StringUnder300{primitives.StringUnder300(wdk.TxStatusUnfail), primitives.StringUnder300(specops.ListActionsSpecOpFailedActionsLabel)},
		Limit:  10,
		Offset: 0,
	})

	// and:
	provider.ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()

	// when:
	err = activeStorage.UnFail(t.Context())

	// then:
	require.NoError(t, err)
	thenDB := testabilities.ThenDBState(t, activeStorage)
	thenDB.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusInvalid)
}

func TestUnFail_Empty_NoItems(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// when: no transactions marked unfail
	err := activeStorage.UnFail(t.Context())

	// then:
	require.NoError(t, err)
}

func TestUnFail_WithMerklePath_CreatedOutputsAreRestoredToSpendable(t *testing.T) {
	// Regression test: when unfail succeeds (merkle path found), the outputs created by
	// the recovered TX must be restored to spendable=true so they can be used downstream.
	// Before this fix, a successful unfail would leave outputs non-spendable if the fix
	// for the failure path had already run.

	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	provider := given.Provider()
	activeStorage := provider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: create a TX and make it fail with double spend
	createActionResult, signedTx := given.Action(activeStorage).
		WithSatoshisToInternalize(10_000).
		WithSatoshisToSend(1_000).
		Created()
	txID := signedTx.TxID().String()
	otherTXID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
	provider.ARC().WhenQueryingTx(txID).WillReturnDoubleSpending(otherTXID)

	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   []primitives.TXIDHexString{},
	})
	require.NoError(t, err)

	// and: mark it for unfail
	_, _ = activeStorage.ListActions(t.Context(), testusers.Alice.AuthID(), wdk.ListActionsArgs{
		Labels: []primitives.StringUnder300{
			primitives.StringUnder300(wdk.TxStatusUnfail),
			primitives.StringUnder300(specops.ListActionsSpecOpFailedActionsLabel),
		},
		Limit: 10,
	})

	// and: mock merkle path present → unfail will succeed
	mp := testutils.MockValidMerklePath(t, txID, 2000)
	provider.ARC().WhenQueryingTx(txID).WillReturnTransactionWithMerklePath(mp)

	// when:
	err = activeStorage.UnFail(t.Context())
	require.NoError(t, err)

	// then: user TX is now unproven (recovered)
	testabilities.ThenDBState(t, activeStorage).
		HasUserTransactionByTxID(testusers.Alice, txID).
		WithStatus(wdk.TxStatusUnproven)

	// and: outputs created by the recovered TX are spendable again
	transactionIDs, err := activeStorage.Repo().FindTransactionIDsByTxID(t.Context(), txID)
	require.NoError(t, err)
	require.Len(t, transactionIDs, 1)

	outputs, err := activeStorage.Repo().FindOutputsByTransactionID(t.Context(), transactionIDs[0])
	require.NoError(t, err)
	require.NotEmpty(t, outputs)

	for _, output := range outputs {
		assert.True(t, output.Spendable,
			"output vout=%d from an unfailed TX must be spendable after successful recovery", output.Vout)
	}
}

func TestUnFail_NoMerklePath_CreatedOutputsRemainNotSpendable(t *testing.T) {
	// Regression test: when unfail fails (no merkle path), the outputs created by the TX
	// must remain spendable=false — they should not become available for spending.

	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	provider := given.Provider()
	activeStorage := provider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: create a TX and make it fail with double spend
	createActionResult, signedTx := given.Action(activeStorage).
		WithSatoshisToInternalize(10_000).
		WithSatoshisToSend(1_000).
		Created()
	txID := signedTx.TxID().String()
	otherTXID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
	provider.ARC().WhenQueryingTx(txID).WillReturnDoubleSpending(otherTXID)

	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   []primitives.TXIDHexString{},
	})
	require.NoError(t, err)

	// and: mark it for unfail
	_, _ = activeStorage.ListActions(t.Context(), testusers.Alice.AuthID(), wdk.ListActionsArgs{
		Labels: []primitives.StringUnder300{
			primitives.StringUnder300(wdk.TxStatusUnfail),
			primitives.StringUnder300(specops.ListActionsSpecOpFailedActionsLabel),
		},
		Limit: 10,
	})

	// and: merkle path NOT found → unfail will cascade back to invalid
	provider.ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()

	// when:
	err = activeStorage.UnFail(t.Context())
	require.NoError(t, err)

	// then: known TX is back to invalid, user TX stays failed
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusInvalid)

	// and: outputs created by the TX remain not spendable
	transactionIDs, err := activeStorage.Repo().FindTransactionIDsByTxID(t.Context(), txID)
	require.NoError(t, err)
	require.Len(t, transactionIDs, 1)

	outputs, err := activeStorage.Repo().FindOutputsByTransactionID(t.Context(), transactionIDs[0])
	require.NoError(t, err)
	require.NotEmpty(t, outputs)

	for _, output := range outputs {
		assert.False(t, output.Spendable,
			"output vout=%d from a TX that failed unfail must not be spendable", output.Vout)
	}
}
