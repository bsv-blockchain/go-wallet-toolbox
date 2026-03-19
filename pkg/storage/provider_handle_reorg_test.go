package storage_test

import (
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	pkgtestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestHandleReorg_InvalidateProofsForOrphanedBlocks(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and: mined tx with block hash
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100)
	txID := txSpec.ID().String()

	// and: sync to get merkle proof
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnWithMindedTx()
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})
	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined().
		WithBlockHash(to.Ptr(testservices.TestBlockHash))

	// when:
	err = activeStorage.HandleReorg(t.Context(), []string{testservices.TestBlockHash})

	// then:
	require.NoError(t, err)

	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusReorg).
		NotMined().
		WithBlockHash(nil).
		WithBlockHeight(nil).
		WithMerkleRoot(nil).
		WithAttempts(0).
		TxNotes(func(then testabilities.TxNotesAssertion) {
			then.Count(2) // first note from sync, second from reorg
			then.Note(history.NotifyTxOfProofHistoryNote, nil, map[string]any{
				"transactionId": uint(1),
			})
			then.Note(history.ReorgInvalidatedProof, nil, map[string]any{
				"orhpaned_block_hash": testservices.TestBlockHash,
				"status_now":          string(wdk.ProvenTxStatusReorg),
			})
		})
}

func TestHandleReorg_DoesNothingWhenNoMatchingBlockHash(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and: mined tx with block hash
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100)
	txID := txSpec.ID().String()

	// and: sync to get merkle proof
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnWithMindedTx()
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})
	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined().
		WithBlockHash(to.Ptr(testservices.TestBlockHash))

	// when:
	err = activeStorage.HandleReorg(t.Context(), []string{"0000000001"})

	// then:
	require.NoError(t, err)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined().
		WithBlockHash(to.Ptr(testservices.TestBlockHash))
}

func TestHandleReorg_MultipleTxsInSameBlock(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and: multiple transactions in the same block
	txSpec1, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	txSpec2, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	txID1 := txSpec1.ID().String()
	txID2 := txSpec2.ID().String()

	// and: sync both to get proofs (same block)
	givenProvider.ARC().WhenQueryingTx(txID1).WillReturnWithMindedTx()
	givenProvider.ARC().WhenQueryingTx(txID2).WillReturnWithMindedTx()
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})
	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	// when:
	err = activeStorage.HandleReorg(t.Context(), []string{testservices.TestBlockHash})

	// then:
	require.NoError(t, err)

	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID1).
		WithStatus(wdk.ProvenTxStatusReorg).
		NotMined().
		WithBlockHash(nil).
		WithBlockHeight(nil).
		WithMerkleRoot(nil).
		WithAttempts(0).
		TxNotes(func(then testabilities.TxNotesAssertion) {
			then.Count(2) // first note from sync, second from reorg
			then.Note(history.NotifyTxOfProofHistoryNote, nil, map[string]any{
				"transactionId": uint(1),
			})
			then.Note(history.ReorgInvalidatedProof, nil, map[string]any{
				"orhpaned_block_hash": testservices.TestBlockHash,
				"status_now":          string(wdk.ProvenTxStatusReorg),
			})
		})
	thenDBState.HasKnownTX(txID2).
		WithStatus(wdk.ProvenTxStatusReorg).
		NotMined().
		WithBlockHash(nil).
		WithBlockHeight(nil).
		WithMerkleRoot(nil).
		WithAttempts(0).
		TxNotes(func(then testabilities.TxNotesAssertion) {
			then.Count(2) // first note from sync, second from reorg
			then.Note(history.NotifyTxOfProofHistoryNote, nil, map[string]any{
				"transactionId": uint(1),
			})
			then.Note(history.ReorgInvalidatedProof, nil, map[string]any{
				"orhpaned_block_hash": testservices.TestBlockHash,
				"status_now":          string(wdk.ProvenTxStatusReorg),
			})
		})
}

func TestHandleReorg_ReorgedTxCanBeReprovenBySyncTask(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and: mined tx with block hash
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	txID := txSpec.ID().String()

	// and: sync to get proof
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnWithMindedTx()
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})
	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	// and:
	err = activeStorage.HandleReorg(t.Context(), []string{pkgtestabilities.TestBlockHash})
	require.NoError(t, err)

	// then:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusReorg)

	// and when: sync task runs again (tx is mined in new block)
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnWithMindedTx()
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList(
		testservices.WithTipBlockHeaderHeight(testservices.TestBlockHeight + 1),
	)
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight + 1),
	})
	_, err = activeStorage.SynchronizeTransactionStatuses(t.Context())

	// then:
	require.NoError(t, err)

	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined()
}
