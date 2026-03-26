package storage_test

import (
	"testing"

	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
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
