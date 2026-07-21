package storage_test

import (
	"testing"

	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/specops"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TestListFailedActionsWithUnfail_ToleratesTransactionWithoutKnownTxRow is a regression
// test for a merge-gating bug introduced by W1-4: UpdateKnownTxStatus now returns a
// %w-wrapped repo.ErrStatusUpdateSkipped when the UPDATE matches zero rows. Before the
// fix, ListFailedActions(unfail=true) propagated that error verbatim for ANY failed
// Transaction whose tx_id has no matching KnownTx row - aborting the whole call, even
// though such "orphan" rows are a known, tolerated shape elsewhere in this codebase
// (FindTransactionIDsForAbort's own filter joins Transaction to KnownTx with a LEFT JOIN
// and defaults via COALESCE(known_txs.status, 'unprocessed') specifically because
// KnownTx-less rows exist in practice).
func TestListFailedActionsWithUnfail_ToleratesTransactionWithoutKnownTxRow(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	provider := given.Provider()
	activeStorage := provider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: a normal failed tx (double spend) - has a tx_id AND a matching KnownTx row
	normalCreateResult, normalSignedTx := given.Action(activeStorage).Created()
	normalTxID := normalSignedTx.TxID().String()
	normalCompetingTxID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
	provider.ARC().WhenQueryingTx(normalTxID).WillReturnDoubleSpending(normalCompetingTxID)

	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(normalCreateResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(normalTxID)),
		RawTx:      normalSignedTx.Bytes(),
		SendWith:   []primitives.TXIDHexString{},
	})
	require.NoError(t, err)

	// and: a second failed tx whose tx_id has NO matching KnownTx row (orphan) - proven
	// possible in production per the abort sweep's own COALESCE-based filter
	orphanCreateResult, orphanSignedTx := given.Action(activeStorage).Created()
	orphanTxID := orphanSignedTx.TxID().String()
	orphanCompetingTxID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
	provider.ARC().WhenQueryingTx(orphanTxID).WillReturnDoubleSpending(orphanCompetingTxID)

	_, err = activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(orphanCreateResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(orphanTxID)),
		RawTx:      orphanSignedTx.Bytes(),
		SendWith:   []primitives.TXIDHexString{},
	})
	require.NoError(t, err)

	// and: delete the orphan tx's KnownTx row, simulating the no-KnownTx-row case.
	// bsv_tx_notes carries a real FK to bsv_known_txes(tx_id) (enforced on Postgres,
	// not on SQLite), so the child notes written by ProcessAction must go first.
	db := activeStorage.Database.DB
	require.NoError(t, db.Exec(`DELETE FROM bsv_tx_notes WHERE tx_id = ?`, orphanTxID).Error)
	require.NoError(t, db.Exec(`DELETE FROM bsv_known_txes WHERE tx_id = ?`, orphanTxID).Error)

	// when: ListFailedActions is invoked with unfail=true (via the ListActions spec-op route)
	result, err := activeStorage.ListActions(t.Context(), testusers.Alice.AuthID(), wdk.ListActionsArgs{
		Labels: []primitives.StringUnder300{
			primitives.StringUnder300(wdk.TxStatusUnfail),
			primitives.StringUnder300(specops.ListActionsSpecOpFailedActionsLabel),
		},
		Limit: 10,
	})

	// then: no error, and BOTH failed actions are listed - the orphan tx's missing
	// KnownTx row must not fail the whole call
	require.NoError(t, err)
	require.EqualValues(t, 2, result.TotalActions)

	txIDs := make([]string, 0, len(result.Actions))
	for _, a := range result.Actions {
		txIDs = append(txIDs, a.TxID)
	}
	assert.ElementsMatch(t, []string{normalTxID, orphanTxID}, txIDs)

	// and: the normal tx's KnownTx was moved to 'unfail' as usual
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(normalTxID).WithStatus(wdk.ProvenTxStatusUnfail)
}

// TestListFailedActions_IncludesAbortedWithDistinctStatus proves an aborted tx
// (never broadcast) surfaces in the failed-actions bucket alongside genuinely failed
// txs, with its raw status preserved so the caller can distinguish retryable-abort
// from permanent-rejection. It also proves Unfail does not re-fail the aborted tx.
func TestListFailedActions_IncludesAbortedWithDistinctStatus(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	provider := given.Provider()
	activeStorage := provider.WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	// and: a rejected (failed) tx
	failedCreate, failedSigned := given.Action(activeStorage).Created()
	failedTxID := failedSigned.TxID().String()
	competingTxID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
	provider.ARC().WhenQueryingTx(failedTxID).WillReturnDoubleSpending(competingTxID)
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx: true, Reference: to.Ptr(failedCreate.Reference),
		TxID: to.Ptr(primitives.TXIDHexString(failedTxID)), RawTx: failedSigned.Bytes(),
		SendWith: []primitives.TXIDHexString{},
	})
	require.NoError(t, err)

	// and: an aborted tx that DOES carry a real TxID and a KnownTx row (in the
	// abortable 'nosend' status), so markActionsForUnfail reaches the new
	// Status != TxStatusFailed guard rather than being filtered out earlier by
	// the pre-existing empty-TxID skip. Without this, the aborted fixture would
	// have TxID=="" and no KnownTx row, and the test would pass identically even
	// if the new guard were deleted - it would not actually pin the guard.
	abortedCreate, abortedSignedTx := given.Action(activeStorage).Created()
	abortedTxID := abortedSignedTx.TxID().String()
	_, err = activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  true,
		Reference: to.Ptr(abortedCreate.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(abortedTxID)),
		RawTx:     abortedSignedTx.Bytes(),
		SendWith:  []primitives.TXIDHexString{},
	})
	require.NoError(t, err)

	// and: confirm the pre-abort KnownTx status really is 'nosend', so the
	// post-list assertion below pins the exact value rather than an assumption
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(abortedTxID).WithStatus(wdk.ProvenTxStatusNoSend)

	_, err = activeStorage.AbortAction(t.Context(), testusers.Alice.AuthID(), wdk.AbortActionArgs{
		Reference: primitives.Base64String(abortedCreate.Reference),
	})
	require.NoError(t, err)

	// when: list the failed-actions bucket (spec-op route), with unfail requested
	result, err := activeStorage.ListActions(t.Context(), testusers.Alice.AuthID(), wdk.ListActionsArgs{
		Labels: []primitives.StringUnder300{
			primitives.StringUnder300(wdk.TxStatusUnfail),
			primitives.StringUnder300(specops.ListActionsSpecOpFailedActionsLabel),
		},
		Limit: 10,
	})

	// then: both are returned, with distinct raw statuses
	require.NoError(t, err)
	require.EqualValues(t, 2, result.TotalActions)
	statuses := map[string]int{}
	for _, a := range result.Actions {
		statuses[a.Status]++
	}
	require.Equal(t, 1, statuses[string(wdk.TxStatusFailed)], "one failed action expected")
	require.Equal(t, 1, statuses[string(wdk.TxStatusAborted)], "one aborted action expected")

	// and: the aborted tx was NOT re-failed by the unfail side-effect
	testabilities.ThenDBState(t, activeStorage).
		HasUserTransactionByReference(testusers.Alice, abortedCreate.Reference).
		WithStatus(wdk.TxStatusAborted)

	// and: crucially, the aborted tx's shared KnownTx row was NOT flipped to
	// 'unfail' - this is what the Status != TxStatusFailed guard in
	// markActionsForUnfail actually prevents (AbortAction itself never touches
	// KnownTx, so it stays at its pre-abort 'nosend' status; if the guard were
	// removed, this would be flipped to 'unfail' and the later UnFail cron would
	// cascade it back to 'failed', re-erasing the aborted status)
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(abortedTxID).WithStatus(wdk.ProvenTxStatusNoSend)
}
