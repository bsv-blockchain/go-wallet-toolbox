package wdk_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// TestProvenTxReqStatusPredicates is the settled semantics for every
// ProvenTxReqStatus predicate. It enumerates ALL 14 statuses against every
// predicate/mapping so the table itself is the contract (W1-6).
//
// Membership sets pinned here:
//   - Sending():           {sending, unsent, nosend, unknown, nonfinal, unprocessed}
//     (terminal invalidTx/doubleSpend intentionally excluded — they fall through to failed)
//   - AlreadySent():       {unmined, callback, unconfirmed, completed}
//     (reorg intentionally excluded — a reorged tx is NOT currently network-accepted)
//   - WasBroadcastStatus(): {unmined, callback, unconfirmed, completed, reorg}
//     (reorg MUST stay — reorg re-sync eligibility rides on was_broadcast)
//   - IsInFlight():         {sending, unsent, unprocessed}
//   - IsTerminalFailure():  {invalidTx, doubleSpend}
//   - IsAcceptedUnproven(): {unmined, callback, unconfirmed}
//   - IsFinalMined():       {completed}
func TestProvenTxReqStatusPredicates(t *testing.T) {
	type expectation struct {
		status               wdk.ProvenTxReqStatus
		sending              bool
		alreadySent          bool
		wasBroadcast         bool
		isInFlight           bool
		isTerminalFailure    bool
		isAcceptedUnproven   bool
		isFinalMined         bool
		standardized         wdk.StandardizedTxStatus
		sendWithResultStatus wdk.SendWithResultStatus
	}

	// One explicit row per status. Every column is stated — no defaults.
	table := []expectation{
		{
			status:               wdk.ProvenTxStatusSending,
			sending:              true,
			alreadySent:          false,
			wasBroadcast:         false,
			isInFlight:           true,
			isTerminalFailure:    false,
			isAcceptedUnproven:   false,
			isFinalMined:         false,
			standardized:         wdk.TxUpdateStatusWaiting,
			sendWithResultStatus: wdk.SendWithResultStatusSending,
		},
		{
			status:               wdk.ProvenTxStatusUnsent,
			sending:              true,
			alreadySent:          false,
			wasBroadcast:         false,
			isInFlight:           true,
			isTerminalFailure:    false,
			isAcceptedUnproven:   false,
			isFinalMined:         false,
			standardized:         wdk.TxUpdateStatusWaiting,
			sendWithResultStatus: wdk.SendWithResultStatusSending,
		},
		{
			status:               wdk.ProvenTxStatusNoSend,
			sending:              true,
			alreadySent:          false,
			wasBroadcast:         false,
			isInFlight:           false,
			isTerminalFailure:    false,
			isAcceptedUnproven:   false,
			isFinalMined:         false,
			standardized:         wdk.TxUpdateStatusWaiting,
			sendWithResultStatus: wdk.SendWithResultStatusSending,
		},
		{
			status:               wdk.ProvenTxStatusUnknown,
			sending:              true,
			alreadySent:          false,
			wasBroadcast:         false,
			isInFlight:           false,
			isTerminalFailure:    false,
			isAcceptedUnproven:   false,
			isFinalMined:         false,
			standardized:         wdk.TxUpdateStatusUnknown,
			sendWithResultStatus: wdk.SendWithResultStatusSending,
		},
		{
			status:               wdk.ProvenTxStatusNonFinal,
			sending:              true,
			alreadySent:          false,
			wasBroadcast:         false,
			isInFlight:           false,
			isTerminalFailure:    false,
			isAcceptedUnproven:   false,
			isFinalMined:         false,
			standardized:         wdk.TxUpdateStatusWaiting,
			sendWithResultStatus: wdk.SendWithResultStatusSending,
		},
		{
			status:               wdk.ProvenTxStatusUnprocessed,
			sending:              true,
			alreadySent:          false,
			wasBroadcast:         false,
			isInFlight:           true,
			isTerminalFailure:    false,
			isAcceptedUnproven:   false,
			isFinalMined:         false,
			standardized:         wdk.TxUpdateStatusWaiting,
			sendWithResultStatus: wdk.SendWithResultStatusSending,
		},
		{
			status:               wdk.ProvenTxStatusUnmined,
			sending:              false,
			alreadySent:          true,
			wasBroadcast:         true,
			isInFlight:           false,
			isTerminalFailure:    false,
			isAcceptedUnproven:   true,
			isFinalMined:         false,
			standardized:         wdk.TxUpdateStatusBroadcasted,
			sendWithResultStatus: wdk.SendWithResultStatusUnproven,
		},
		{
			status:               wdk.ProvenTxStatusCallback,
			sending:              false,
			alreadySent:          true,
			wasBroadcast:         true,
			isInFlight:           false,
			isTerminalFailure:    false,
			isAcceptedUnproven:   true,
			isFinalMined:         false,
			standardized:         wdk.TxUpdateStatusBroadcasted,
			sendWithResultStatus: wdk.SendWithResultStatusUnproven,
		},
		{
			status:               wdk.ProvenTxStatusUnconfirmed,
			sending:              false,
			alreadySent:          true,
			wasBroadcast:         true,
			isInFlight:           false,
			isTerminalFailure:    false,
			isAcceptedUnproven:   true,
			isFinalMined:         false,
			standardized:         wdk.TxUpdateStatusBroadcasted,
			sendWithResultStatus: wdk.SendWithResultStatusUnproven,
		},
		{
			status:               wdk.ProvenTxStatusCompleted,
			sending:              false,
			alreadySent:          true,
			wasBroadcast:         true,
			isInFlight:           false,
			isTerminalFailure:    false,
			isAcceptedUnproven:   false,
			isFinalMined:         true,
			standardized:         wdk.TxUpdateStatusMined,
			sendWithResultStatus: wdk.SendWithResultStatusUnproven,
		},
		{
			status:               wdk.ProvenTxStatusInvalid,
			sending:              false,
			alreadySent:          false,
			wasBroadcast:         false,
			isInFlight:           false,
			isTerminalFailure:    true,
			isAcceptedUnproven:   false,
			isFinalMined:         false,
			standardized:         wdk.TxUpdateStatusInvalidTx,
			sendWithResultStatus: wdk.SendWithResultStatusFailed,
		},
		{
			status:               wdk.ProvenTxStatusDoubleSpend,
			sending:              false,
			alreadySent:          false,
			wasBroadcast:         false,
			isInFlight:           false,
			isTerminalFailure:    true,
			isAcceptedUnproven:   false,
			isFinalMined:         false,
			standardized:         wdk.TxUpdateStatusDoubleSpend,
			sendWithResultStatus: wdk.SendWithResultStatusFailed,
		},
		{
			status:               wdk.ProvenTxStatusUnfail,
			sending:              false,
			alreadySent:          false,
			wasBroadcast:         false,
			isInFlight:           false,
			isTerminalFailure:    false,
			isAcceptedUnproven:   false,
			isFinalMined:         false,
			standardized:         wdk.TxUpdateStatusWaiting,
			sendWithResultStatus: wdk.SendWithResultStatusFailed,
		},
		{
			status:               wdk.ProvenTxStatusReorg,
			sending:              false,
			alreadySent:          false,
			wasBroadcast:         true,
			isInFlight:           false,
			isTerminalFailure:    false,
			isAcceptedUnproven:   false,
			isFinalMined:         false,
			standardized:         wdk.TxUpdateStatusWaiting,
			sendWithResultStatus: wdk.SendWithResultStatusFailed,
		},
	}

	// Guard: the table must cover every declared ProvenTxReqStatus. If a new
	// status is added to the enum, this test must be extended to cover it.
	allStatuses := []wdk.ProvenTxReqStatus{
		wdk.ProvenTxStatusSending,
		wdk.ProvenTxStatusUnsent,
		wdk.ProvenTxStatusNoSend,
		wdk.ProvenTxStatusUnknown,
		wdk.ProvenTxStatusNonFinal,
		wdk.ProvenTxStatusUnprocessed,
		wdk.ProvenTxStatusUnmined,
		wdk.ProvenTxStatusCallback,
		wdk.ProvenTxStatusUnconfirmed,
		wdk.ProvenTxStatusCompleted,
		wdk.ProvenTxStatusInvalid,
		wdk.ProvenTxStatusDoubleSpend,
		wdk.ProvenTxStatusUnfail,
		wdk.ProvenTxStatusReorg,
	}
	assert.Len(t, table, len(allStatuses), "predicate table must cover all ProvenTxReqStatus values")
	covered := make(map[wdk.ProvenTxReqStatus]bool, len(table))
	for _, row := range table {
		covered[row.status] = true
	}
	for _, s := range allStatuses {
		assert.Truef(t, covered[s], "status %q is not covered by the predicate table", s)
	}

	for _, row := range table {
		t.Run(string(row.status), func(t *testing.T) {
			assert.Equalf(t, row.sending, row.status.Sending(), "Sending(%q)", row.status)
			assert.Equalf(t, row.alreadySent, row.status.AlreadySent(), "AlreadySent(%q)", row.status)
			assert.Equalf(t, row.wasBroadcast, row.status.WasBroadcastStatus(), "WasBroadcastStatus(%q)", row.status)
			assert.Equalf(t, row.isInFlight, row.status.IsInFlight(), "IsInFlight(%q)", row.status)
			assert.Equalf(t, row.isTerminalFailure, row.status.IsTerminalFailure(), "IsTerminalFailure(%q)", row.status)
			assert.Equalf(t, row.isAcceptedUnproven, row.status.IsAcceptedUnproven(), "IsAcceptedUnproven(%q)", row.status)
			assert.Equalf(t, row.isFinalMined, row.status.IsFinalMined(), "IsFinalMined(%q)", row.status)
			assert.Equalf(t, row.standardized, row.status.ToStandardizedStatus(), "ToStandardizedStatus(%q)", row.status)
			assert.Equalf(t, row.sendWithResultStatus, row.status.SendWithResultStatus(), "SendWithResultStatus(%q)", row.status)
		})
	}
}

// TestTxStatusMappings pins ToUTXOStatus and ToStandardizedStatus for every TxStatus
// value. The coverage guard forces this table to be extended whenever a new TxStatus
// is added to the enum.
func TestTxStatusMappings(t *testing.T) {
	type expectation struct {
		status       wdk.TxStatus
		utxo         wdk.UTXOStatus
		standardized wdk.StandardizedTxStatus
	}

	table := []expectation{
		{wdk.TxStatusCompleted, wdk.UTXOStatusMined, wdk.TxUpdateStatusMined},
		{wdk.TxStatusFailed, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusInvalidTx},
		{wdk.TxStatusUnprocessed, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusWaiting},
		{wdk.TxStatusSending, wdk.UTXOStatusSending, wdk.TxUpdateStatusWaiting},
		{wdk.TxStatusUnproven, wdk.UTXOStatusUnproven, wdk.TxUpdateStatusBroadcasted},
		{wdk.TxStatusUnsigned, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusWaiting},
		{wdk.TxStatusNoSend, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusWaiting},
		{wdk.TxStatusNonFinal, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusWaiting},
		{wdk.TxStatusUnfail, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusWaiting},
		{wdk.TxStatusAborted, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusFailed},
	}

	allStatuses := []wdk.TxStatus{
		wdk.TxStatusCompleted, wdk.TxStatusFailed, wdk.TxStatusUnprocessed,
		wdk.TxStatusSending, wdk.TxStatusUnproven, wdk.TxStatusUnsigned,
		wdk.TxStatusNoSend, wdk.TxStatusNonFinal, wdk.TxStatusUnfail,
		wdk.TxStatusAborted,
	}
	assert.Len(t, table, len(allStatuses), "mapping table must cover all TxStatus values")

	for _, row := range table {
		t.Run(string(row.status), func(t *testing.T) {
			assert.Equalf(t, row.utxo, row.status.ToUTXOStatus(), "ToUTXOStatus(%q)", row.status)
			assert.Equalf(t, row.standardized, row.status.ToStandardizedStatus(), "ToStandardizedStatus(%q)", row.status)
		})
	}
}
