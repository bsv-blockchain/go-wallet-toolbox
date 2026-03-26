package storage_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities/nosendtest"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
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
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})

	// when:
	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())

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

func TestSynchronizeManyTxs(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	const count = 150

	// and:
	txIDs := make([]string, count)
	for i := range count {
		txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
		givenProvider.ARC().WhenQueryingTx(txSpec.ID().String()).WillReturnWithMindedTx()
		txIDs[i] = txSpec.ID().String()
	}

	// and:
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})

	// when:
	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())

	// then:
	require.NoError(t, err)

	// and:
	thenDBState := testabilities.ThenDBState(t, activeStorage)

	for _, txID := range txIDs {
		thenDBState.
			HasKnownTX(txID).
			WithStatus(wdk.ProvenTxStatusCompleted).
			IsMined()

		thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).
			WithStatus(wdk.TxStatusCompleted)
	}

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
	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())

	// then: synchronization should succeed but no transactions should be processed
	// because we can't get status for txIDs when WhatsOnChain is unreachable
	require.NoError(t, err)

	// and: transaction should remain unmined since we couldn't filter transactions
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.
		HasKnownTX(txSpec.ID().String()).
		WithStatus(wdk.ProvenTxStatusUnmined).
		NotMined()

	thenDBState.HasUserTransactionByTxID(testusers.Alice, txSpec.ID().String()).
		WithStatus(wdk.TxStatusUnproven)
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
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})

	// when:
	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())

	// then:
	require.NoError(t, err)

	// and:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txSpec.ID().String()).
		NotMined()

	// and:
	require.Equal(t, 1, servicesSniffer.CountCallsByRegex(fmt.Sprintf("arc(.*)tx\\/%s", txSpec.ID())))

	// when:
	_, err = activeStorage.SynchronizeTransactionStatuses(t.Context())

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
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})

	// when:
	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())

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
	_, err = activeStorage.SynchronizeTransactionStatuses(t.Context())

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
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})

	// when:
	for attempt := range defs.DefaultSynchronizeTxStatuses().MaxAttempts {
		_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
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
			givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
				ExpectBlockHash:   testservices.TestBlockHash,
				ExpectBlockHeight: int64(testservices.TestBlockHeight),
			})
			test.setupARCMock(arcQueryFixture)

			// when:
			_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())

			// then:
			require.NoError(t, err)

			// NOTE: Error is not returned, because this action tries to synchronize all transactions and skips those that are not found or have no Merkle Path.
		})
	}
}

func TestSynchronizeTxForSameHeightDifferentHash(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	txID := txSpec.ID().String()

	// and: setup mocks for first sync
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnWithMindedTx()
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})

	// when: first sync - transaction gets mined
	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	// then: transaction is mined
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined()

	// when: reorg happens - invalidate proofs for the block
	err = activeStorage.HandleReorg(t.Context(), []string{testservices.TestBlockHash})
	require.NoError(t, err)

	// then: transaction status is now 'reorg' and proof is invalidated
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusReorg).
		NotMined()

	// given: new chain tip has SAME height but DIFFERENT hash (competing block won)
	const newBlockHash = "000000000000000001885e0c6c302cbbacf927e1b5cf7884588973e72f8b9999"
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnWithMindedTx()
	givenProvider.WhatsOnChain().
		OnTipBlockHeaderWillRespondWithOneElementList(
			testservices.WithTipBlockHeaderHeight(testservices.TestBlockHeight),
			testservices.WithTipBlockHeaderHash(newBlockHash),
		)
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   newBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})

	// when: second sync - same height but different hash should not skip
	_, err = activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	// then: transaction is mined again (re-proven in the new winning block)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined()
}

func TestSynchronizeTxNoSendBroadcastedExternally(t *testing.T) {
	t.Run("no send tx broadcasted externally is marked as mined", func(t *testing.T) {
		// given:
		const inputSatoshis = 6

		given, when, _, cleanup := nosendtest.New(t, testusers.Alice)
		defer cleanup()

		activeStorage := given.ActiveProvider()

		// and:
		given.UserOwnsGivenUTXOsToSpend(inputSatoshis)

		// when:
		when.WillSendSats(1).CreateAndProcessNoSendAction(nil)

		// and:
		noSendTxID := when.NoSendTxs()[0]
		given.Provider().ARC().WhenQueryingTx(noSendTxID).WillReturnWithMindedTx()
		given.Provider().WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
			ExpectBlockHash:   testservices.TestBlockHash,
			ExpectBlockHeight: int64(testservices.TestBlockHeight),
		})

		_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())

		// then:
		require.NoError(t, err)

		// and:
		testabilities.ThenDBState(t, activeStorage).
			HasKnownTX(noSendTxID).
			IsMined()

		testabilities.ThenFunds(t, testusers.Alice, activeStorage).
			ShouldBeAbleToReserveSatoshis(inputSatoshis - 2) // -1 satoshi sent - 1 satoshi fee
	})

	t.Run("no send tx not found externally is left as unproven", func(t *testing.T) {
		// given:
		const inputSatoshis = 6

		given, when, _, cleanup := nosendtest.New(t, testusers.Alice)
		defer cleanup()

		activeStorage := given.ActiveProvider()

		// and:
		given.UserOwnsGivenUTXOsToSpend(inputSatoshis)

		// and:
		given.Provider().WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
			ExpectBlockHash:   testservices.TestBlockHash,
			ExpectBlockHeight: int64(testservices.TestBlockHeight),
		})

		// when:
		when.WillSendSats(1).CreateAndProcessNoSendAction(nil)
		_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())

		// then:
		require.NoError(t, err)

		// and:
		noSendTxID := when.NoSendTxs()[0]
		testabilities.ThenDBState(t, activeStorage).
			HasKnownTX(noSendTxID).
			NotMined()

		testabilities.ThenFunds(t, testusers.Alice, activeStorage).
			ShouldNotBeAbleToReserveSatoshis(inputSatoshis - 2)
	})
}
