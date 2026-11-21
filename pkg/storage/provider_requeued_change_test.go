package storage_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// The change of a tx the network rejected must stay out of non-delayed funding
// until the network is seen to have the tx, not merely until a broadcaster accepts
// it again. In the incident this guards against, the broadcaster accepted every
// rebroadcast and rejected the tx later, so a demotion that a POST acceptance undid
// left the change spendable for most of each retry cycle.

const requeuedChangeTopUp = 100_000

// a small amount the tx's change can cover on its own
const requeuedChangeSpend = 1_000

func TestRequeuedTxChangeStaysSendingAcrossAcceptedRebroadcast(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	_, signedTx := given.Action(activeStorage).
		WithSatoshisToInternalize(requeuedChangeTopUp).
		Processed()
	txID := signedTx.TxID().String()

	// and: its change funds a non-delayed CreateAction:
	testabilities.ThenFunds(t, testusers.Alice, activeStorage).
		ShouldBeAbleToReserveSatoshis(requeuedChangeSpend)

	// and: the network does not know the tx, and the broadcaster rejects it:
	given.Provider().WhatsOnChain().WillRespondOnTxStatusNotFound()
	_, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:   txID,
		Status: rejectedStatus,
	})
	require.NoError(t, err)

	// then: the tx and its change are 'sending':
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnsent).WithRebroadcastAttempts(1)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusSending)
	thenDBState.ChangeUTXOsOfTx(testusers.Alice, txID).AllWithStatus(wdk.UTXOStatusSending)

	// and: a non-delayed CreateAction cannot fund from it, a delayed one still can:
	thenFunds := testabilities.ThenFunds(t, testusers.Alice, activeStorage)
	thenFunds.ShouldNotBeAbleToReserveSatoshis(requeuedChangeSpend)
	require.True(t, thenFunds.CanReserveSatoshisWithDelayedBroadcast(requeuedChangeSpend),
		"a delayed broadcast may fund from 'sending' change")

	// and: aborting that delayed action restored its input as 'sending', not 'unproven'
	// (abort re-derives the tier from the parent tx's status):
	thenDBState.ChangeUTXOsOfTx(testusers.Alice, txID).AllWithStatus(wdk.UTXOStatusSending)

	// when: the tx is rebroadcast and the broadcaster accepts it again:
	_, err = activeStorage.SendWaitingTransactions(t.Context(), -time.Minute)
	require.NoError(t, err)

	// then: it is back in flight, but acceptance is not evidence - it stays 'sending':
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusSending)
	thenDBState.ChangeUTXOsOfTx(testusers.Alice, txID).AllWithStatus(wdk.UTXOStatusSending)
	thenFunds.ShouldNotBeAbleToReserveSatoshis(requeuedChangeSpend)

	// when: the network is seen to have the tx:
	_, err = activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
		TxID:   txID,
		Status: "SEEN_ON_NETWORK",
	})
	require.NoError(t, err)

	// then: the tx and its change are 'unproven' again and fund non-delayed creates:
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusUnproven)
	thenDBState.ChangeUTXOsOfTx(testusers.Alice, txID).AllWithStatus(wdk.UTXOStatusUnproven)
	thenFunds.ShouldBeAbleToReserveSatoshis(requeuedChangeSpend)
}

// A late or replayed RECEIVED (or SENT_TO_NETWORK / ACCEPTED_BY_NETWORK) for a
// requeued tx advances it back in flight, but is not evidence the network has it.
func TestRequeuedTxChangeStaysSendingOnLateReceivedEvent(t *testing.T) {
	for _, status := range []string{"RECEIVED", "SENT_TO_NETWORK", "ACCEPTED_BY_NETWORK"} {
		t.Run(status, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()
			activeStorage := given.Provider().
				WithRandomizer(randomizer.NewTestRandomizer()).
				GORM()

			_, signedTx := given.Action(activeStorage).
				WithSatoshisToInternalize(requeuedChangeTopUp).
				Processed()
			txID := signedTx.TxID().String()

			// and: a REJECTED event without evidence requeued it:
			given.Provider().WhatsOnChain().WillRespondOnTxStatusNotFound()
			_, err := activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
				TxID:   txID,
				Status: rejectedStatus,
			})
			require.NoError(t, err)

			// when:
			_, err = activeStorage.ProcessExternalTxStatusUpdate(t.Context(), wdk.BroadcastStatusEvent{
				TxID:   txID,
				Status: status,
			})

			// then: in flight again, still 'sending':
			require.NoError(t, err)
			thenDBState := testabilities.ThenDBState(t, activeStorage)
			thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
			thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusSending)
			thenDBState.ChangeUTXOsOfTx(testusers.Alice, txID).AllWithStatus(wdk.UTXOStatusSending)
			testabilities.ThenFunds(t, testusers.Alice, activeStorage).
				ShouldNotBeAbleToReserveSatoshis(requeuedChangeSpend)
		})
	}
}

// A tx that was never requeued is promoted on acceptance exactly as before: the
// evidence rule applies only once the network has lost or rejected a tx.
func TestNeverRequeuedTxIsPromotedOnReceivedEvent(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	_, signedTx := given.Action(activeStorage).
		WithSatoshisToInternalize(requeuedChangeTopUp).
		Processed()
	txID := signedTx.TxID().String()

	// then: broadcast and accepted, never requeued: 'unproven', and it funds creates:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithRebroadcastAttempts(0)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusUnproven)
	thenDBState.ChangeUTXOsOfTx(testusers.Alice, txID).AllWithStatus(wdk.UTXOStatusUnproven)
	testabilities.ThenFunds(t, testusers.Alice, activeStorage).
		ShouldBeAbleToReserveSatoshis(requeuedChangeSpend)
}
