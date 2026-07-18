package storage_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// These tests cover the aggregation of conflicting broadcast results.
//
// Reported scenario: when many chained transactions are broadcast in sequence,
// WhatsOnChain (which receives the raw tx) answers "missing inputs" or
// "txn-mempool-conflict" for a tx whose parent has not reached its mempool yet,
// while ARC accepts the very same tx. Treating that single report as a terminal
// double spend marks the tx as failed and releases its inputs back to spendable,
// after which the wallet builds REAL double spends and the cascade corrupts the
// wallet beyond repair.

// When WoC rejects with "missing inputs" but ARC accepts the tx AND the network
// status check confirms the tx is known, the double-spend verdict is a false
// positive and the tx must be treated as successfully broadcast.
func TestBroadcastConflict_MissingInputsButTxKnown_IsSuccess(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: ARC accepts the tx (default fixture behavior), WoC lags behind:
	given.Provider().WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, "unexpected response code 500: Missing inputs")
	given.Provider().WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{})

	// when:
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// then:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusUnproven)
}

// When WoC rejects with "missing inputs", ARC accepts the tx, but the network
// status check cannot (yet) see the tx, the result must stay retryable
// (sending) instead of becoming a terminal failure that releases the inputs.
// A later retry then completes the broadcast.
func TestBroadcastConflict_MissingInputsWithSuccess_TxUnknown_IsRetryable(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: ARC accepts the tx (default fixture behavior), WoC lags behind and does not know the tx:
	given.Provider().WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, "unexpected response code 500: Missing inputs")
	given.Provider().WhatsOnChain().WillRespondOnTxStatusNotFound()

	// when:
	_, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// then: the tx is NOT failed, it stays in a retryable state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusSending)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusSending)

	// and when: WoC has caught up by the time of the retry:
	given.Provider().WhatsOnChain().WillRespondWithBroadcast(http.StatusOK, `{"txid":"`+txID+`"}`)
	given.Provider().WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{})
	_, err := activeStorage.SendWaitingTransactions(t.Context(), -time.Minute)

	// then: the retry completes the broadcast:
	require.NoError(t, err)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusUnproven)
}

// A "missing inputs" report with no competing transactions, no successful
// broadcaster and an unknown network status carries no positive evidence of a
// double spend - it is indistinguishable from propagation lag, so the tx must
// stay retryable rather than be terminally failed.
func TestBroadcastConflict_MissingInputsOnly_TxUnknown_IsRetryable(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: WoC reports missing inputs and does not know the tx:
	given.Provider().WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, "unexpected response code 500: Missing inputs")
	given.Provider().WhatsOnChain().WillRespondOnTxStatusNotFound()

	// when: ARC fails as well (WillFailOnBroadcast):
	_, signedTx := given.Action(activeStorage).WillFailOnBroadcast().Processed()
	txID := signedTx.TxID().String()

	// then:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusSending)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusSending)
}

// A mempool conflict reported by WoC, with no successful broadcaster and the tx
// unknown to the network, is a confirmed double spend and must keep failing the
// transaction (the pre-existing terminal behavior).
func TestBroadcastConflict_MempoolConflict_TxUnknown_StaysFailed(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: WoC reports a mempool conflict and does not know the tx:
	given.Provider().WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, "unexpected response code 500: 258: txn-mempool-conflict")
	given.Provider().WhatsOnChain().WillRespondOnTxStatusNotFound()

	// when: ARC fails as well (WillFailOnBroadcast):
	_, signedTx := given.Action(activeStorage).WillFailOnBroadcast().Processed()
	txID := signedTx.TxID().String()

	// then:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusDoubleSpend)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusFailed)
}
