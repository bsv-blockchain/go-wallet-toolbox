package storage_test

import (
	"fmt"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

const wocEndpointRegex = "whatsonchain(.*)headers"

func TestSynchronizeTx(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and:
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	givenProvider.ARC().WhenQueryingTx(txSpec.ID().String()).WillReturnWithMindedTx()
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()

	// when:
	err := activeStorage.SynchronizeTransactionStatuses(t.Context())

	// then:
	require.NoError(t, err)

	// and:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.
		HasKnownTX(txSpec.ID().String()).
		WithStatus(wdk.ProvenTxStatusCompleted).
		WithAttempts(0).
		IsMined().
		TxNotes(func(then testabilities.TxNotesAssertion) {
			then.
				Count(1).
				Note("notifyTxOfProof", nil, map[string]any{
					"transactionId": uint(1),
				})
		})

	thenDBState.HasUserTransactionByTxID(testusers.Alice, txSpec.ID().String()).
		WithStatus(wdk.TxStatusCompleted)

	// and:
	require.Equal(t, 1, givenProvider.ServicesSniffer().CountCallsByRegex(wocEndpointRegex))
}

func TestSynchronizeTxEvenIfChainTipIsUnreachable(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and:
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	givenProvider.ARC().WhenQueryingTx(txSpec.ID().String()).WillReturnWithMindedTx()

	// NOTE: WhatsOnChain is unreachable, so we simulate that the chain tip is not available
	_ = givenProvider.WhatsOnChain().WillBeUnreachable()

	// when:
	err := activeStorage.SynchronizeTransactionStatuses(t.Context())

	// and:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.
		HasKnownTX(txSpec.ID().String()).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined()

	thenDBState.HasUserTransactionByTxID(testusers.Alice, txSpec.ID().String()).
		WithStatus(wdk.TxStatusCompleted)

	// then:
	require.NoError(t, err)
}

func TestSynchronizeTxForTheSameBlockHeightTwice(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and:
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	servicesSniffer := givenProvider.ServicesSniffer()
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()

	// when:
	err := activeStorage.SynchronizeTransactionStatuses(t.Context())

	// then:
	require.NoError(t, err)

	// and:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txSpec.ID().String()).
		NotMined()

	// and:
	require.Equal(t, 1, servicesSniffer.CountCallsByRegex(fmt.Sprintf("arc(.*)tx\\/%s", txSpec.ID())))

	// when:
	err = activeStorage.SynchronizeTransactionStatuses(t.Context())

	// then:
	require.NoError(t, err)

	// and:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txSpec.ID().String()).
		NotMined()

	// and:
	require.Equal(t, 2, servicesSniffer.CountCallsByRegex(wocEndpointRegex))
	require.Equal(t, 1, servicesSniffer.CountCallsByRegex(fmt.Sprintf("arc(.*)tx\\/%s", txSpec.ID())))
	// NOTE: The second call should not trigger a request for the transaction, because the block height is the same
}

func TestSynchronizeTxForTwoDifferentBlockHeights(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and:
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	servicesSniffer := givenProvider.ServicesSniffer()
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()

	// when:
	err := activeStorage.SynchronizeTransactionStatuses(t.Context())

	// then:
	require.NoError(t, err)

	// and:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txSpec.ID().String()).
		NotMined()

	// and:
	require.Equal(t, 1, servicesSniffer.CountCallsByRegex(wocEndpointRegex))
	require.Equal(t, 1, servicesSniffer.CountCallsByRegex(fmt.Sprintf("arc(.*)tx\\/%s", txSpec.ID())))

	// given:
	givenProvider.ARC().WhenQueryingTx(txSpec.ID().String()).WillReturnWithMindedTx()
	givenProvider.WhatsOnChain().
		OnTipBlockHeaderWillRespondWithOneElementList(
			testservices.WithTipBlockHeaderHeight(testservices.TestBlockHeight + 1),
		)

	// when:
	err = activeStorage.SynchronizeTransactionStatuses(t.Context())

	// then:
	require.NoError(t, err)

	// and:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txSpec.ID().String()).
		IsMined()

	// and:
	require.Equal(t, 2, servicesSniffer.CountCallsByRegex(wocEndpointRegex))
	require.Equal(t, 2, servicesSniffer.CountCallsByRegex(fmt.Sprintf("arc(.*)tx\\/%s", txSpec.ID())))
	// NOTE: The second call should also trigger a request for the transaction, because the block height is different
}

func TestFailedSyncExceedsMaxAttempts(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and:
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	givenProvider.ARC().WhenQueryingTx(txSpec.ID().String()).WillReturnTransactionWithoutMerklePath()

	// when:
	for attempt := range defs.DefaultSynchronizeTxStatuses().MaxAttempts {
		err := activeStorage.SynchronizeTransactionStatuses(t.Context())
		require.NoError(t, err)

		// then:
		testabilities.ThenDBState(t, activeStorage).HasKnownTX(txSpec.ID().String()).WithAttempts(attempt + 1)
	}

	// and:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txSpec.ID().String()).
		WithStatus(wdk.ProvenTxStatusInvalid).
		WithAttempts(defs.DefaultSynchronizeTxStatuses().MaxAttempts).
		NotMined()
}

func TestSynchronizeTxEdgeCases(t *testing.T) {
	tests := map[string]struct {
		setupARCMock func(arcQueryFixture testservices.ARCQueryFixture)
	}{
		"ARC returns transaction without MerklePath": {
			setupARCMock: func(arcQueryFixture testservices.ARCQueryFixture) {
				arcQueryFixture.WillReturnTransactionWithoutMerklePath()
			},
		},
		"ARC returns no body": {
			setupARCMock: func(arcQueryFixture testservices.ARCQueryFixture) {
				arcQueryFixture.WillReturnNoBody()
			},
		},
		"ARC is unreachable": {
			setupARCMock: func(arcQueryFixture testservices.ARCQueryFixture) {
				arcQueryFixture.WillBeUnreachable()
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			// given:
			givenProvider := given.Provider()
			activeStorage := givenProvider.GORM()

			// and:
			txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

			// and:
			arcQueryFixture := givenProvider.ARC().WhenQueryingTx(txSpec.ID().String())
			givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()
			test.setupARCMock(arcQueryFixture)

			// when:
			err := activeStorage.SynchronizeTransactionStatuses(t.Context())

			// then:
			require.NoError(t, err)

			// NOTE: Error is not returned, because this action tries to synchronize all transactions and skips those that are not found or have no Merkle Path.
		})
	}
}
