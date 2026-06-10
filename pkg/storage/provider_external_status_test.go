package storage_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// These tests cover applying broadcaster-pushed lifecycle updates (e.g. Arcade SSE events)
// to the storage. The external events must never corrupt the wallet state:
//   - MINED events apply through the same atomic UpdateKnownTxAsMined flow as polling,
//   - REJECTED events are re-verified against the network before any terminal failure
//     (same machinery as the false-double-spend protection for broadcast results).

const (
	externalEventBlockHeight = 2000
	externalEventBlockHash   = "000000000000000001885e0c6c302cbbacf927e1b5cf7884588973e72f8b1234"
	externalNoteWhat         = "externalBroadcastStatus"
)

// A MINED event carrying a valid merkle path must complete the transaction
// without any merkle path fetch from the services.
func TestExternalStatusMinedWithMerklePathInEvent(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: a broadcast transaction (status unmined):
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// and: a merkle path for the tx whose root is confirmed for its height:
	merklePath := testutils.MockValidMerklePath(t, txID, externalEventBlockHeight)
	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	require.NoError(t, err)
	given.Provider().BHS().OnMerkleRootVerifyResponse(externalEventBlockHeight, merkleRoot, testabilities.BHSMerkleRootConfirmed)

	// when:
	results, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		EventID:     "1000",
		TxID:        txID,
		Status:      "MINED",
		BlockHash:   externalEventBlockHash,
		BlockHeight: externalEventBlockHeight,
		MerklePath:  merklePath.Hex(),
	})

	// then:
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, txID, results[0].TxID)
	require.Equal(t, wdk.ProvenTxStatusCompleted, results[0].Status)
	require.Equal(t, uint32(externalEventBlockHeight), results[0].BlockHeight)
	require.Equal(t, externalEventBlockHash, results[0].BlockHash)
	require.Equal(t, merkleRoot, results[0].MerkleRoot)
	require.NotNil(t, results[0].MerklePath)

	// and:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined()
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).
		WithStatus(wdk.TxStatusCompleted)
}

// A MINED event with an invalid merkle root (not confirmed for the height) must not
// change the stored state.
func TestExternalStatusMinedWithInvalidMerkleRootIsRejected(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// and: the root verification reports the root as NOT confirmed:
	merklePath := testutils.MockValidMerklePath(t, txID, externalEventBlockHeight)
	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	require.NoError(t, err)
	given.Provider().BHS().OnMerkleRootVerifyResponse(externalEventBlockHeight, merkleRoot, "INVALID")

	// when:
	results, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:        txID,
		Status:      "MINED",
		BlockHash:   externalEventBlockHash,
		BlockHeight: externalEventBlockHeight,
		MerklePath:  merklePath.Hex(),
	})

	// then:
	require.Error(t, err)
	require.Empty(t, results)

	// and: the stored state is unchanged:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusUnmined).
		NotMined()
}

// A MINED event without a merkle path falls back to fetching the proof from the services.
func TestExternalStatusMinedWithoutPathFallsBackToServices(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	givenProvider := given.Provider()
	activeStorage := givenProvider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// and: the services know the merkle path for the tx:
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnWithMindedTx()

	// when:
	results, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:   txID,
		Status: "MINED",
	})

	// then:
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, wdk.ProvenTxStatusCompleted, results[0].Status)

	// and:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined()
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).
		WithStatus(wdk.TxStatusCompleted)
}

// A SEEN_ON_NETWORK event advances an in-flight (sending) transaction to the same
// post-broadcast status broadcastTxs sets after a successful broadcast (unmined),
// and records a history note about the event.
func TestExternalStatusSeenAdvancesSendingToUnmined(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: a broadcast that ends up retryable (sending): WoC lags behind, ARC accepts,
	// network status check does not see the tx yet:
	given.Provider().WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, "unexpected response code 500: Missing inputs")
	given.Provider().WhatsOnChain().WillRespondOnTxStatusNotFound()
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// and: precondition - the tx is in the sending state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusSending)

	// when:
	results, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		EventID: "2000",
		TxID:    txID,
		Status:  "SEEN_ON_NETWORK",
	})

	// then:
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, wdk.ProvenTxStatusUnmined, results[0].Status)

	// and:
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusUnproven)

	// and: the event is recorded in the tx history:
	// NOTE: the repo applies only the TxID condition when it is set, so filter on "what" in memory.
	notes, err := activeStorage.TxNoteEntity().Read().TxID(txID).Find(t.Context())
	require.NoError(t, err)
	externalNotes := 0
	for _, note := range notes {
		if note.What == externalNoteWhat {
			externalNotes++
			require.EqualValues(t, "SEEN_ON_NETWORK", note.Attributes["status"])
		}
	}
	require.Equal(t, 1, externalNotes)
}

// A RECEIVED event advances an in-flight (sending) transaction exactly like SEEN_*:
// it is the first broadcaster acknowledgement that the tx was accepted.
func TestExternalStatusReceivedAdvancesSendingToUnmined(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: a broadcast that ends up retryable (sending): WoC lags behind, ARC accepts,
	// network status check does not see the tx yet:
	given.Provider().WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, "unexpected response code 500: Missing inputs")
	given.Provider().WhatsOnChain().WillRespondOnTxStatusNotFound()
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// and: precondition - the tx is in the sending state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusSending)

	// when:
	results, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		EventID: "2100",
		TxID:    txID,
		Status:  "RECEIVED",
	})

	// then:
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, wdk.ProvenTxStatusUnmined, results[0].Status)

	// and:
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusUnproven)
}

// An event with a status string this storage does not understand is a no-op, not an error
// (the external stream may grow new statuses any time).
func TestExternalStatusUnsupportedStatusIsNoOp(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: a successfully broadcast transaction (status unmined):
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// when:
	results, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:   txID,
		Status: "DOUBLE_SPEND_ATTEMPTED_MAYBE",
	})

	// then:
	require.NoError(t, err)
	require.Empty(t, results)

	// and: the stored state is unchanged:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusUnproven)
}

// A MINED event without a merkle path, where the services do not have the proof either,
// changes nothing - the polling safety net picks the tx up later.
func TestExternalStatusMinedWithoutPathAndNoProofIsLeftToPolling(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	givenProvider := given.Provider()
	activeStorage := givenProvider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// and: the services know the tx but have no merkle path for it yet:
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()

	// when:
	results, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:   txID,
		Status: "MINED",
	})

	// then: no error and no result - nothing was applied:
	require.NoError(t, err)
	require.Empty(t, results)

	// and: the stored state is unchanged:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusUnmined).
		NotMined()
}

// A MINED event whose merkle path comes without a block hash must not persist an empty
// block hash on the KnownTx - the event-supplied path is ignored and the proof is taken
// from the services instead.
func TestExternalStatusMinedWithPathButNoBlockHashFallsBackToServices(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	givenProvider := given.Provider()
	activeStorage := givenProvider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// and: the services know the merkle path for the tx:
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnWithMindedTx()

	// when: the event carries a merkle path but no block hash:
	merklePath := testutils.MockValidMerklePath(t, txID, externalEventBlockHeight)
	results, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:        txID,
		Status:      "MINED",
		BlockHeight: externalEventBlockHeight,
		MerklePath:  merklePath.Hex(),
	})

	// then: the tx is completed from the services' proof, never with an empty block hash:
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, wdk.ProvenTxStatusCompleted, results[0].Status)
	require.NotEmpty(t, results[0].BlockHash)

	// and:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined()
}

// A SEEN_ON_NETWORK event for a transaction already at (or past) the post-broadcast
// status is a no-op.
func TestExternalStatusSeenAlreadyBroadcastIsNoOp(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: a successfully broadcast transaction (status unmined):
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// when:
	results, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:   txID,
		Status: "SEEN_ON_MULTIPLE_NODES",
	})

	// then:
	require.NoError(t, err)
	require.Empty(t, results)

	// and:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusUnmined)
}

// A REJECTED event for a transaction the network already knows is a false positive
// and must NOT fail the transaction.
func TestExternalStatusRejectedButTxKnownToNetworkIsNotFailed(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: a successfully broadcast transaction (status unmined):
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// and: the network knows the tx:
	given.Provider().WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{})

	// when:
	_, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:         txID,
		Status:       "REJECTED",
		ExtraInfo:    "double spend attempted",
		CompetingTxs: []string{"27a53423aa3e5d5c46bf30be53a9998dd247daf758847f244f82d430be71de6e"},
	})

	// then:
	require.NoError(t, err)

	// and: the tx is NOT failed:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusUnproven)
}

// A REJECTED event with competing transactions, where the network does not know the tx,
// is a confirmed double spend and must go through the existing terminal failure path.
func TestExternalStatusRejectedWithCompetingTxsConfirmedIsFailed(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: a successfully broadcast transaction (status unmined):
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// and: the network does not know the tx:
	given.Provider().WhatsOnChain().WillRespondOnTxStatusNotFound()

	// when:
	results, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:         txID,
		Status:       "REJECTED",
		ExtraInfo:    "double spend attempted",
		CompetingTxs: []string{"27a53423aa3e5d5c46bf30be53a9998dd247daf758847f244f82d430be71de6e"},
	})

	// then:
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, wdk.ProvenTxStatusDoubleSpend, results[0].Status)

	// and:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusDoubleSpend)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusFailed)
}

// A REJECTED event without competing transactions carries no positive evidence of a
// double spend - even when the network does not (yet) know the tx, it must stay
// retryable rather than be terminally failed (polling safety net keeps it alive).
func TestExternalStatusRejectedWithoutEvidenceStaysRetryable(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: a successfully broadcast transaction (status unmined):
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// and: the network does not know the tx:
	given.Provider().WhatsOnChain().WillRespondOnTxStatusNotFound()

	// when:
	_, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:   txID,
		Status: "REJECTED",
	})

	// then:
	require.NoError(t, err)

	// and: the tx is NOT failed:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusUnproven)
}

// An event for a txid this storage never stored is a no-op (SSE is instance scoped,
// but be defensive).
func TestExternalStatusUnknownTxIDIsNoOp(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// when:
	results, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:   "27a53423aa3e5d5c46bf30be53a9998dd247daf758847f244f82d430be71de6e",
		Status: "MINED",
	})

	// then:
	require.NoError(t, err)
	require.Empty(t, results)
}

// An event for a transaction already in a terminal status is a no-op.
func TestExternalStatusTerminalStatusIsNoOp(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: a transaction completed via an external MINED event:
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	merklePath := testutils.MockValidMerklePath(t, txID, externalEventBlockHeight)
	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	require.NoError(t, err)
	given.Provider().BHS().OnMerkleRootVerifyResponse(externalEventBlockHeight, merkleRoot, testabilities.BHSMerkleRootConfirmed)

	_, err = activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:        txID,
		Status:      "MINED",
		BlockHash:   externalEventBlockHash,
		BlockHeight: externalEventBlockHeight,
		MerklePath:  merklePath.Hex(),
	})
	require.NoError(t, err)

	// when: a late REJECTED event arrives for the completed tx:
	results, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:         txID,
		Status:       "REJECTED",
		CompetingTxs: []string{"27a53423aa3e5d5c46bf30be53a9998dd247daf758847f244f82d430be71de6e"},
	})

	// then:
	require.NoError(t, err)
	require.Empty(t, results)

	// and: the tx stays completed:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusCompleted).IsMined()
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusCompleted)
}

// Reproduces the TOCTOU race the terminal-failure path must survive: the tx completes
// (MINED) WHILE a REJECTED event is being re-verified against the network - i.e. after
// the handler's initial terminal-status check has already passed. The guarded, atomic
// failure path must then write NOTHING: failing the Transaction or resurrecting its spent
// inputs on a completed tx is the false-double-spend corruption class from 6addd9e.
func TestExternalStatusRejectedRacingCompletionLeavesCompletedTxUntouched(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: a successfully broadcast transaction (status unmined):
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// and: a valid merkle path the racing MINED event will carry:
	merklePath := testutils.MockValidMerklePath(t, txID, externalEventBlockHeight)
	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	require.NoError(t, err)
	given.Provider().BHS().OnMerkleRootVerifyResponse(externalEventBlockHeight, merkleRoot, testabilities.BHSMerkleRootConfirmed)

	// and: the network re-check (confirmDoubleSpends) reports the tx as unknown, and the
	// tx completes concurrently while that re-check is in flight (the responder applies a
	// MINED event before answering - after the REJECTED handler's terminal check passed):
	completedConcurrently := false
	given.Provider().WhatsOnChain().Transport().RegisterResponder(http.MethodPost, `=~txs/status\z`,
		func(req *http.Request) (*http.Response, error) {
			if !completedConcurrently {
				completedConcurrently = true
				_, minedErr := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
					TxID:        txID,
					Status:      "MINED",
					BlockHash:   externalEventBlockHash,
					BlockHeight: externalEventBlockHeight,
					MerklePath:  merklePath.Hex(),
				})
				require.NoError(t, minedErr)
			}

			var body struct {
				Txids []string `json:"txids"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(http.StatusBadRequest, "bad request"), nil
			}
			respItems := []map[string]any{}
			for _, txid := range body.Txids {
				respItems = append(respItems, map[string]any{"txid": txid, "error": "unknown"})
			}
			respBytes, _ := json.Marshal(respItems)
			resp := httpmock.NewStringResponse(http.StatusOK, string(respBytes))
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		})

	// when: the REJECTED event (with double-spend evidence) finishes its verification:
	results, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:         txID,
		Status:       "REJECTED",
		ExtraInfo:    "double spend attempted",
		CompetingTxs: []string{"27a53423aa3e5d5c46bf30be53a9998dd247daf758847f244f82d430be71de6e"},
	})

	// then: the race actually happened and the rejection was swallowed without changes:
	require.True(t, completedConcurrently)
	require.NoError(t, err)
	require.Empty(t, results)

	// and: the completed tx and its UTXO state are untouched:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusCompleted).IsMined()
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusCompleted)
}

// GetKeyValue/SetKeyValue expose the key_value table for small instance state
// (e.g. the SSE replay cursor).
func TestKeyValueRoundTrip(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().GORM()

	// when: reading a missing key:
	value, found, err := activeStorage.GetKeyValue(t.Context(), "arcade_sse_last_event_id")

	// then:
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, value)

	// when: setting and reading the key back:
	err = activeStorage.SetKeyValue(t.Context(), "arcade_sse_last_event_id", []byte("1749550000000000000"))
	require.NoError(t, err)

	value, found, err = activeStorage.GetKeyValue(t.Context(), "arcade_sse_last_event_id")

	// then:
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("1749550000000000000"), value)

	// when: overwriting the key:
	err = activeStorage.SetKeyValue(t.Context(), "arcade_sse_last_event_id", []byte("1749550000000000001"))
	require.NoError(t, err)

	value, found, err = activeStorage.GetKeyValue(t.Context(), "arcade_sse_last_event_id")

	// then:
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("1749550000000000001"), value)
}
