package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestInternalize_BeefVerification(t *testing.T) {
	tests := map[string]struct {
		ok      bool
		err     error
		wantErr string
	}{
		"verifier refuses the BEEF": {ok: false, wantErr: "provided beef is not valid"},
		"verifier fails":            {ok: true, err: errInternalizeBoom, wantErr: "failed to verify beef"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			f := newInternalizeFixture(t)
			f.beefVerifier.ok, f.beefVerifier.err = test.ok, test.err
			args, _ := paymentArgs(t)

			// when:
			result, err := f.action.Internalize(t.Context(), internalizeTestUserID, &args)

			// then:
			require.ErrorContains(t, err, test.wantErr)
			assert.Nil(t, result)
			assert.Empty(t, f.scriptsVerifier.checked, "scripts must not be checked for a refused BEEF")
			assert.Zero(t, f.uow.calls, "nothing may be stored for a refused BEEF")
		})
	}
}

func TestInternalize_ScriptVerification(t *testing.T) {
	tests := map[string]struct {
		ok      bool
		err     error
		wantErr string
	}{
		"scripts are invalid":   {ok: false, wantErr: "scripts are not valid for tx"},
		"script verifier fails": {ok: true, err: errInternalizeBoom, wantErr: "script verification failed for tx"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			f := newInternalizeFixture(t)
			f.scriptsVerifier.ok, f.scriptsVerifier.err = test.ok, test.err
			args, _ := paymentArgs(t)

			// when:
			result, err := f.action.Internalize(t.Context(), internalizeTestUserID, &args)

			// then:
			require.ErrorContains(t, err, test.wantErr)
			assert.Nil(t, result)
			assert.Zero(t, f.uow.calls, "nothing may be stored when scripts do not verify")
		})
	}
}

func TestInternalize_ChecksScriptsOfUnminedTxsOnly(t *testing.T) {
	// given:
	f := newInternalizeFixture(t)
	args, txID := paymentArgs(t)

	// when:
	_, err := f.action.Internalize(t.Context(), internalizeTestUserID, &args)

	// then: the mined source tx in the BEEF is skipped, the unmined payment is checked
	require.NoError(t, err)
	assert.Equal(t, []string{txID}, f.scriptsVerifier.checked)
}

func TestInternalize_NewTx_BroadcastFailureIsSoft(t *testing.T) {
	// given: storage does not know the tx, and the broadcast claim fails
	f := newInternalizeFixture(t)
	args, txID := paymentArgs(t)

	// when:
	result, err := f.action.Internalize(t.Context(), internalizeTestUserID, &args)

	// then: the internalize is accepted and reports the tx as still sending
	require.NoError(t, err)
	assert.Equal(t, &wdk.InternalizeActionResult{
		Accepted: true,
		IsMerge:  false,
		TxID:     txID,
		Satoshis: int64(fixtures.ExpectedValueToInternalize),
		SendWithResults: []wdk.SendWithResult{{
			TxID:   primitives.TXIDHexString(txID),
			Status: wdk.SendWithResultStatusSending,
		}},
	}, result)

	// and: the broadcast was attempted for exactly this tx
	assert.Equal(t, [][]string{{txID}}, f.knownTx.claimed)

	// and: stored as an unsent known tx and a sending transaction
	require.Len(t, f.knownTx.upserted, 1)
	assert.Equal(t, txID, f.knownTx.upserted[0].TxID)
	assert.Equal(t, wdk.ProvenTxStatusUnsent, f.knownTx.upserted[0].Status)
	assert.Equal(t, []wdk.ProvenTxReqStatus{
		wdk.ProvenTxStatusCompleted, wdk.ProvenTxStatusUnmined, wdk.ProvenTxStatusSending, wdk.ProvenTxStatusUnsent,
	}, f.knownTx.upserted[0].SkipForStatuses)
	assert.NotEmpty(t, f.knownTx.upserted[0].InputBeef, "the anchored BEEF is stored")

	require.Len(t, f.txRepo.created, 1)
	assert.Equal(t, wdk.TxStatusSending, f.txRepo.created[0].Status)
	assert.Equal(t, int64(fixtures.ExpectedValueToInternalize), f.txRepo.created[0].Satoshis)
}

func TestInternalize_NewTx_KnownStatusDecidesBroadcast(t *testing.T) {
	tests := map[string]struct {
		known         wdk.ProvenTxReqStatus
		wantKnownTx   wdk.ProvenTxReqStatus
		wantTxStatus  wdk.TxStatus
		wantBroadcast bool
	}{
		"already accepted by the network": {
			known: wdk.ProvenTxStatusUnmined, wantKnownTx: wdk.ProvenTxStatusUnmined, wantTxStatus: wdk.TxStatusUnproven,
		},
		"already completed": {
			known: wdk.ProvenTxStatusCompleted, wantKnownTx: wdk.ProvenTxStatusUnmined, wantTxStatus: wdk.TxStatusUnproven,
		},
		"in flight on another path": {
			known: wdk.ProvenTxStatusSending, wantKnownTx: wdk.ProvenTxStatusUnsent, wantTxStatus: wdk.TxStatusSending,
		},
		"known but never sent": {
			known: wdk.ProvenTxStatusUnknown, wantKnownTx: wdk.ProvenTxStatusUnsent, wantTxStatus: wdk.TxStatusSending,
			wantBroadcast: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			f := newInternalizeFixture(t)
			args, txID := paymentArgs(t)
			f.knownTx.statuses[txID] = test.known

			// when:
			result, err := f.action.Internalize(t.Context(), internalizeTestUserID, &args)

			// then:
			require.NoError(t, err)
			require.Len(t, f.knownTx.upserted, 1)
			assert.Equal(t, test.wantKnownTx, f.knownTx.upserted[0].Status)
			require.Len(t, f.txRepo.created, 1)
			assert.Equal(t, test.wantTxStatus, f.txRepo.created[0].Status)

			if test.wantBroadcast {
				assert.Len(t, f.knownTx.claimed, 1)
				assert.NotEmpty(t, result.SendWithResults)
			} else {
				assert.Empty(t, f.knownTx.claimed, "must not re-broadcast")
				assert.Empty(t, result.SendWithResults)
			}
		})
	}
}

func TestInternalize_MissingChangeBasket(t *testing.T) {
	// given:
	f := newInternalizeFixture(t)
	f.baskets.changeBasket = nil
	args, _ := paymentArgs(t)

	// when:
	result, err := f.action.Internalize(t.Context(), internalizeTestUserID, &args)

	// then:
	require.ErrorContains(t, err, "basket for change (default) not found")
	assert.Nil(t, result)
	assert.Empty(t, f.txRepo.created)
}

func TestInternalize_Merge_RefusesDisallowedStatus(t *testing.T) {
	for _, status := range []wdk.TxStatus{
		wdk.TxStatusFailed, wdk.TxStatusAborted, wdk.TxStatusUnprocessed,
		wdk.TxStatusUnsigned, wdk.TxStatusNonFinal, wdk.TxStatusUnfail,
	} {
		t.Run(string(status), func(t *testing.T) {
			// given:
			f := newInternalizeFixture(t)
			f.givenStoredTx(status)
			args, _ := paymentArgs(t)

			// when:
			result, err := f.action.Internalize(t.Context(), internalizeTestUserID, &args)

			// then:
			require.ErrorContains(t, err, "target transaction of internalizeAction has invalid status")
			assert.Nil(t, result)
			assert.Empty(t, f.outputs.saved)
			assert.Empty(t, f.txRepo.labels)
		})
	}
}

func TestInternalize_Merge_ChangeUTXOStatusFollowsTxStatus(t *testing.T) {
	tests := map[wdk.TxStatus]wdk.UTXOStatus{
		wdk.TxStatusCompleted: wdk.UTXOStatusMined,
		wdk.TxStatusUnproven:  wdk.UTXOStatusUnproven,
		wdk.TxStatusSending:   wdk.UTXOStatusSending,
	}
	for txStatus, wantUTXOStatus := range tests {
		t.Run(string(txStatus), func(t *testing.T) {
			// given:
			f := newInternalizeFixture(t)
			f.givenStoredTx(txStatus)
			args, txID := paymentArgs(t)

			// when:
			result, err := f.action.Internalize(t.Context(), internalizeTestUserID, &args)

			// then: merged, without storing a new tx or broadcasting
			require.NoError(t, err)
			assert.Equal(t, &wdk.InternalizeActionResult{
				Accepted: true,
				IsMerge:  true,
				TxID:     txID,
				Satoshis: int64(fixtures.ExpectedValueToInternalize),
			}, result)
			assert.Empty(t, f.txRepo.created)
			assert.Empty(t, f.knownTx.upserted)
			assert.Empty(t, f.knownTx.claimed)

			// and: labels go to the stored tx, the change basket is ensured
			require.Len(t, f.txRepo.labels, 1)
			assert.Equal(t, internalizeLabelsCall{
				userID: internalizeTestUserID, transactionID: 42, labels: []string{"label1", "label2"},
			}, f.txRepo.labels[0])
			assert.Equal(t, []string{wdk.BasketNameForChange}, f.baskets.ensured)

			// and: the change output gets a UTXO in the status that matches the stored tx
			require.Len(t, f.outputs.saved, 1)
			require.Len(t, f.outputs.saved[0], 1)
			saved := f.outputs.saved[0][0]
			require.NotNil(t, saved.UserUTXO)
			assert.Equal(t, wantUTXOStatus, saved.UserUTXO.Status)
			assert.Equal(t, uint64(fixtures.ExpectedValueToInternalize), saved.UserUTXO.Satoshis)
		})
	}
}

// A nosend tx may be merged into (isAllowedMergeStatus), but a change output then has no UTXO
// status to map to, so the merge fails. Pinned as current behavior.
func TestInternalize_Merge_NoSendChangeOutputFails(t *testing.T) {
	// given:
	f := newInternalizeFixture(t)
	f.givenStoredTx(wdk.TxStatusNoSend)
	args, _ := paymentArgs(t)

	// when:
	result, err := f.action.Internalize(t.Context(), internalizeTestUserID, &args)

	// then:
	require.ErrorContains(t, err, "unsupported transaction status for UTXO: nosend")
	assert.Nil(t, result)
	assert.Empty(t, f.outputs.saved)
}

func TestInternalize_Merge_RefusesZeroSatoshiChangeOutput(t *testing.T) {
	// given:
	f := newInternalizeFixture(t)
	f.givenStoredTx(wdk.TxStatusUnproven)
	args, _ := zeroSatoshiPaymentArgs(t)

	// when:
	result, err := f.action.Internalize(t.Context(), internalizeTestUserID, &args)

	// then:
	require.ErrorContains(t, err, "change output with zero satoshis")
	assert.Nil(t, result)
	assert.Empty(t, f.outputs.saved)
}

func TestSendWithResultsFromReview(t *testing.T) {
	const txID = primitives.TXIDHexString("aa")
	tests := map[wdk.ReviewActionResultStatus]wdk.SendWithResultStatus{
		wdk.ReviewActionResultStatusSuccess:      wdk.SendWithResultStatusUnproven,
		wdk.ReviewActionResultStatusServiceError: wdk.SendWithResultStatusSending,
		wdk.ReviewActionResultStatusDoubleSpend:  wdk.SendWithResultStatusFailed,
		wdk.ReviewActionResultStatusInvalidTx:    wdk.SendWithResultStatusFailed,
		"unexpected":                             wdk.SendWithResultStatusSending,
	}
	for reviewStatus, want := range tests {
		t.Run(string(reviewStatus), func(t *testing.T) {
			got := sendWithResultsFromReview([]wdk.ReviewActionResult{{TxID: txID, Status: reviewStatus}})

			assert.Equal(t, []wdk.SendWithResult{{TxID: txID, Status: want}}, got)
		})
	}
}
