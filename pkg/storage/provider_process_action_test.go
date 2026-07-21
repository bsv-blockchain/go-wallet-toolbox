package storage_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

const nLockTimeThreshold = uint32(500_000_000)

func TestProcessActionHappyPath(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()

	// and:
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

	// when:
	result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusUnproven, sendWithResult.Status)

	require.Len(t, result.NotDelayedResults, 1)
	reviewActionResult := result.NotDelayedResults[0]
	assert.Equal(t, txID, string(reviewActionResult.TxID))
	assert.Equal(t, wdk.ReviewActionResultStatusSuccess, reviewActionResult.Status)
	assert.Empty(t, reviewActionResult.CompetingTxs)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		NotMined().
		WithStatus(wdk.ProvenTxStatusUnmined).
		WithAttempts(1).
		HasRawTx().
		TxNotes(func(then testabilities.TxNotesAssertion) {
			then.
				Count(4).
				Note("processAction", to.Ptr(testusers.Alice.ID), nil).
				Note("postBeefSuccess", nil, map[string]any{
					"name": "ARC",
				}).
				Note("postBeefError", nil, map[string]any{
					"name": "WhatsOnChain",
				}).
				Note("aggregateResults", nil, map[string]any{
					"aggStatus":         "success",
					"doubleSpendCount":  0,
					"serviceErrorCount": 1,
					"statusErrorCount":  0,
					"status_now":        "unmined",
					"successCount":      1,
				})
		})

	thenDBState.HasUserTransactionByReference(testusers.Alice, *args.Reference).
		WithTxID(txID).WithStatus(wdk.TxStatusUnproven)
}

func TestProcessActionTwice(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()

	// and:
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

	// when:
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	// when:
	args.IsNewTx = false
	result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusUnproven, sendWithResult.Status)

	require.Empty(t, result.NotDelayedResults)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		NotMined().
		WithStatus(wdk.ProvenTxStatusUnmined).
		HasRawTx()

	thenDBState.HasUserTransactionByReference(testusers.Alice, *args.Reference).
		WithTxID(txID).WithStatus(wdk.TxStatusUnproven)
}

func TestProcessAction_DelayedBroadcast(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).WithDelayedBroadcast().Created()
	txID := signedTx.TxID().String()

	// and:
	args := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  true,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   []primitives.TXIDHexString{},
	}

	// when:
	result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusSending, sendWithResult.Status)

	require.Empty(t, result.NotDelayedResults)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.WaitForTxStatusByReference(testusers.Alice, *args.Reference, wdk.TxStatusUnproven, 5*time.Second)

	// and db state:
	thenDBState.HasKnownTX(txID).
		NotMined().
		WithStatus(wdk.ProvenTxStatusUnmined).
		HasRawTx().
		TxNotes(func(then testabilities.TxNotesAssertion) {
			then.
				Count(4).
				Note("processAction", to.Ptr(testusers.Alice.ID), nil).
				Note("postBeefSuccess", nil, map[string]any{
					"name": "ARC",
				}).
				Note("postBeefError", nil, map[string]any{
					"name": "WhatsOnChain",
				}).
				Note("aggregateResults", nil, map[string]any{
					"aggStatus":         "success",
					"doubleSpendCount":  0,
					"serviceErrorCount": 1,
					"statusErrorCount":  0,
					"status_now":        "unmined",
					"successCount":      1,
				})
		})

	thenDBState.HasUserTransactionByReference(testusers.Alice, *args.Reference).
		WithTxID(txID).WithStatus(wdk.TxStatusUnproven)
}

func TestProcessAction_DelayedBroadcastForManyTransactions(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	const count = 100
	createActionResults := make([]*wdk.StorageCreateActionResult, count)
	signedTxs := make([]*transaction.Transaction, count)
	for i := 0; i < count; i++ {
		satoshisToInternalize := uint64(1000 + i) // this makes transactions different
		createActionResults[i], signedTxs[i] = given.Action(activeStorage).
			WithDelayedBroadcast().
			WithSatoshisToInternalize(satoshisToInternalize).
			WithSatoshisToSend(1).
			Created()
	}

	processActionArgs := make([]wdk.ProcessActionArgs, count)
	for i := 0; i < count; i++ {
		txID := signedTxs[i].TxID().String()
		processActionArgs[i] = wdk.ProcessActionArgs{
			IsNewTx:    true,
			IsSendWith: false,
			IsNoSend:   false,
			IsDelayed:  true,
			Reference:  to.Ptr(createActionResults[i].Reference),
			TxID:       to.Ptr(primitives.TXIDHexString(txID)),
			RawTx:      signedTxs[i].Bytes(),
			SendWith:   []primitives.TXIDHexString{},
		}
	}

	// when:
	var err error
	results := make([]*wdk.ProcessActionResult, count)
	for i := 0; i < count; i++ {
		results[i], err = activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), processActionArgs[i])
		require.NoError(t, err)
	}

	// then:
	for i := 0; i < count; i++ {
		txID := signedTxs[i].TxID().String()
		require.Len(t, results[i].SendWithResults, 1)
		sendWithResult := results[i].SendWithResults[0]
		assert.Equal(t, txID, string(sendWithResult.TxID))
		assert.Equal(t, wdk.SendWithResultStatusSending, sendWithResult.Status)

		require.Empty(t, results[i].NotDelayedResults)
	}

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	for i := 0; i < count; i++ {
		txID := signedTxs[i].TxID().String()

		thenDBState.WaitForTxStatusByReference(testusers.Alice, createActionResults[i].Reference, wdk.TxStatusUnproven, 5*time.Second)

		thenDBState.HasKnownTX(txID).
			NotMined().
			WithStatus(wdk.ProvenTxStatusUnmined).
			HasRawTx()

		thenDBState.HasUserTransactionByReference(testusers.Alice, createActionResults[i].Reference).
			WithTxID(txID).WithStatus(wdk.TxStatusUnproven)
	}
}

func TestProcessActionErrorCases(t *testing.T) {
	tests := map[string]struct {
		argsModifier func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs
	}{
		"IsNewTx set to false for not stored tx": {
			argsModifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.IsNewTx = false
				return args
			},
		},
		"not existing reference": {
			argsModifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.Reference = to.Ptr("not-existing-reference")
				return args
			},
		},
		"tx id missmatch": {
			argsModifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				otherID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
				args.TxID = to.Ptr(primitives.TXIDHexString(otherID))
				return args
			},
		},
		"empty raw tx": {
			argsModifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.RawTx = []byte{}
				return args
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()
			activeStorage := given.Provider().
				WithRandomizer(randomizer.NewTestRandomizer()).
				GORM()

			// and:
			createActionResult, signedTx := given.Action(activeStorage).Created()
			txID := signedTx.TxID().String()

			// and:
			args := test.argsModifier(wdk.ProcessActionArgs{
				IsNewTx:    false,
				IsSendWith: false,
				IsNoSend:   false,
				IsDelayed:  false,
				Reference:  to.Ptr(createActionResult.Reference),
				TxID:       to.Ptr(primitives.TXIDHexString(txID)),
				RawTx:      signedTx.Bytes(),
				SendWith:   []primitives.TXIDHexString{},
			})

			// when:
			_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

			// then:
			require.Error(t, err)
		})
	}
}

func TestProcessActionDoubleSpending(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	givenProvider := given.Provider()
	activeStorage := givenProvider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()

	// and:
	otherTXID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnDoubleSpending(otherTXID)

	// and:
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

	// when:
	result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusFailed, sendWithResult.Status)

	require.Len(t, result.NotDelayedResults, 1)
	reviewActionResult := result.NotDelayedResults[0]
	assert.Equal(t, txID, string(reviewActionResult.TxID))
	assert.Equal(t, wdk.ReviewActionResultStatusDoubleSpend, reviewActionResult.Status)

	require.Len(t, reviewActionResult.CompetingTxs, 1)
	assert.Equal(t, otherTXID, reviewActionResult.CompetingTxs[0])
}

func TestProcessActionARCReturnNoBody(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	givenProvider := given.Provider()
	activeStorage := givenProvider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()

	// and:
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnNoBody()

	// and:
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

	// when:
	result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusSending, sendWithResult.Status)

	require.Len(t, result.NotDelayedResults, 1)
	reviewActionResult := result.NotDelayedResults[0]
	assert.Equal(t, txID, string(reviewActionResult.TxID))
	assert.Equal(t, wdk.ReviewActionResultStatusServiceError, reviewActionResult.Status)
	assert.Empty(t, reviewActionResult.CompetingTxs)
}

func TestProcessAction_ResendAfterError(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	const (
		satoshisToInternalize = 5000
		satoshisToSend        = 1000
		ownedSatoshisAfterTx  = satoshisToInternalize - satoshisToSend - 1
	)

	// and:
	createActionResult, signedTx := given.Action(activeStorage).
		WithSatoshisToInternalize(satoshisToInternalize).
		WithSatoshisToSend(satoshisToSend).
		Created()
	txID := signedTx.TxID().String()

	// and:
	scriptsVerifyMockError := fmt.Errorf("mock scripts verifier error")
	given.Provider().ScriptsVerifier().WillReturnError(scriptsVerifyMockError)

	// when:
	args := wdk.ProcessActionArgs{
		IsNewTx:   true,
		Reference: to.Ptr(createActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:     signedTx.Bytes(),
	}
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.Error(t, err)
	require.ErrorIs(t, err, scriptsVerifyMockError)

	// and db state: transaction is automatically aborted on pre-broadcast failure
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		NotMined().
		WithStatus(wdk.ProvenTxStatusUnprocessed).
		WithAttempts(0).
		HasRawTx()

	thenDBState.HasUserTransactionByReference(testusers.Alice, createActionResult.Reference).
		WithTxID(txID).WithStatus(wdk.TxStatusAborted)

	// and: funds are released because the pre-broadcast abort unreserves UTXOs
	testabilities.ThenFunds(t, testusers.Alice, activeStorage).
		ShouldBeAbleToReserveSatoshis(ownedSatoshisAfterTx)
}

// TestServiceErrorBroadcastCreatesNoSpendableChange guards P5 (Decision Record v1, W2-1): when a
// broadcast comes back as an aggregated service error (no positive evidence either way - not a
// success, not a confirmed double spend, not invalid), the transaction's change output must NOT
// become spendable. Making it spendable on a bare service error would let the wallet spend coins
// backing a transaction that might still turn out to be a double spend or invalid, stranding the
// spender with nothing behind those funds.
func TestServiceErrorBroadcastCreatesNoSpendableChange(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	givenProvider := given.Provider()
	activeStorage := givenProvider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	const (
		satoshisToInternalize = 5000
		satoshisToSend        = 1000
		ownedSatoshisAfterTx  = satoshisToInternalize - satoshisToSend - 1
	)

	// and:
	createActionResult, signedTx := given.Action(activeStorage).
		WithSatoshisToInternalize(satoshisToInternalize).
		WithSatoshisToSend(satoshisToSend).
		Created()
	txID := signedTx.TxID().String()

	// and: ARC cannot confirm anything about this tx (empty response body) - a bare service
	// error with no positive evidence of success, double spend, or invalidity.
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnNoBody()

	// when:
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
	result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)
	require.Len(t, result.NotDelayedResults, 1)
	assert.Equal(t, wdk.ReviewActionResultStatusServiceError, result.NotDelayedResults[0].Status)

	// and: both the known tx and the user's transaction stay retryable (sending), never proven:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusSending)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusSending)

	// and: P5 - no network evidence means the change must NOT become spendable yet.
	//
	// NOTE on the query below: CreateAction eagerly inserts a bsv_user_utxos row for every
	// change output at creation time (see repo/transactions.go:makeNewOutput), well before any
	// broadcast - with utxo_status = "" (Unknown). That row's existence is therefore NOT the
	// signal to check (it exists in both the buggy and fixed behavior). The signal is whether
	// CreateUTXOForSpendableOutputsByTxID (gated by `spendable` in singleTxBroadcastResult) has
	// upserted it to a funder-recognized status. Before this fix, the ServiceError branch upserted
	// utxo_status to "sending" - which IS selectable by the funder in HighAssurance mode
	// (FindNotReservedUTXOsForUpdate's includeSending option) even though nothing here proves the
	// broadcast will ever succeed. So the row must still carry no recognized status.
	var activatedUTXORows int64
	require.NoError(t, activeStorage.Database.DB.Raw(
		`
		SELECT count(*) FROM bsv_user_utxos uu
		JOIN bsv_outputs o ON o.id = uu.output_id
		JOIN bsv_transactions tx ON tx.id = o.transaction_id
		WHERE tx.tx_id = ? AND uu.utxo_status != ''`,
		txID,
	).Scan(&activatedUTXORows).Error)
	assert.Zerof(t, activatedUTXORows,
		"expected no ACTIVATED (funder-recognized status) user_utxos rows for tx %s change after a service-error broadcast, got %d", txID, activatedUTXORows)

	// and: the funder pool cannot reserve the change - it is not (yet) counted as spendable funds.
	testabilities.ThenFunds(t, testusers.Alice, activeStorage).
		ShouldNotBeAbleToReserveSatoshis(ownedSatoshisAfterTx)

	// and: cross-table invariants still hold.
	testabilities.AssertStorageInvariants(t, activeStorage.Database.DB)
}

// TestServiceErrorChangeBecomesSpendableAfterRetrySuccess guards the recovery side of P5: once a
// later retry succeeds, the change that was withheld on the service error becomes spendable - the
// funds are gated (not permanently stranded) until there is real network evidence for the tx.
func TestServiceErrorChangeBecomesSpendableAfterRetrySuccess(t *testing.T) {
	// given: same scenario as TestServiceErrorBroadcastCreatesNoSpendableChange - the tx is stuck
	// in a service error, and its change is not yet spendable.
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	givenProvider := given.Provider()
	activeStorage := givenProvider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	const (
		satoshisToInternalize = 5000
		satoshisToSend        = 1000
		ownedSatoshisAfterTx  = satoshisToInternalize - satoshisToSend - 1
	)

	createActionResult, signedTx := given.Action(activeStorage).
		WithSatoshisToInternalize(satoshisToInternalize).
		WithSatoshisToSend(satoshisToSend).
		Created()
	txID := signedTx.TxID().String()

	givenProvider.ARC().WhenQueryingTx(txID).WillReturnNoBody()

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

	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusSending)

	// sanity: not spendable yet (mirrors TestServiceErrorBroadcastCreatesNoSpendableChange).
	testabilities.ThenFunds(t, testusers.Alice, activeStorage).
		ShouldNotBeAbleToReserveSatoshis(ownedSatoshisAfterTx)

	// when: ARC now confirms the tx is known to the network and a SendWaiting retry re-posts it.
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()
	_, err = activeStorage.SendWaitingTransactions(t.Context(), 0)

	// then:
	require.NoError(t, err)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)

	// and: recovery is not stranded - the change's user_utxos row is now ACTIVATED (upserted to a
	// funder-recognized status, see the NOTE in TestServiceErrorBroadcastCreatesNoSpendableChange).
	var activatedUTXORows int64
	require.NoError(t, activeStorage.Database.DB.Raw(
		`
		SELECT count(*) FROM bsv_user_utxos uu
		JOIN bsv_outputs o ON o.id = uu.output_id
		JOIN bsv_transactions tx ON tx.id = o.transaction_id
		WHERE tx.tx_id = ? AND uu.utxo_status != ''`,
		txID,
	).Scan(&activatedUTXORows).Error)
	assert.Positive(t, activatedUTXORows, "expected ACTIVATED user_utxos row(s) for tx %s change after retry success", txID)

	// ... and the funder pool can now reserve it.
	testabilities.ThenFunds(t, testusers.Alice, activeStorage).
		ShouldBeAbleToReserveSatoshis(ownedSatoshisAfterTx)

	// and: cross-table invariants still hold.
	testabilities.AssertStorageInvariants(t, activeStorage.Database.DB)
}

func TestProcessActionNLockTimeIsFinalSuccess(t *testing.T) {
	tests := map[string]struct {
		setupService func(given testabilities.StorageFixture)
		lockTime     uint32
		sequences    []uint32
		description  string
	}{
		"zero locktime is always final": {
			setupService: func(given testabilities.StorageFixture) {
				// No special setup needed for zero locktime
			},
			lockTime:    0,
			sequences:   []uint32{0, 1, 2},
			description: "zero locktime should always be final",
		},
		"all inputs max sequence shortcut": {
			setupService: func(given testabilities.StorageFixture) {
				// No height check needed when all inputs have max sequence
			},
			lockTime:    700_000,
			sequences:   []uint32{testutils.MaxSeq, testutils.MaxSeq, testutils.MaxSeq},
			description: "all max sequence inputs should be final regardless of locktime",
		},
		"block height locktime final": {
			setupService: func(given testabilities.StorageFixture) {
				const currentHeight = uint32(800_000)
				given.Provider().WhatsOnChain().WillRespondWithChainInfo(http.StatusOK, currentHeight)
			},
			lockTime:    799_999,
			sequences:   []uint32{0},
			description: "locktime less than current height should be final",
		},
		"timestamp locktime final": {
			setupService: func(given testabilities.StorageFixture) {
				// No special setup needed for timestamp validation
			},
			lockTime:    uint32(time.Now().Unix() - 3600), //nolint:gosec // unix timestamp fits in uint32
			sequences:   []uint32{0},
			description: "past timestamp locktime should be final",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			test.setupService(given)
			// These tests are supposed to check nLockTime and not scripts verification so we mock it here
			given.Provider().ScriptsVerifier().WillReturnBool(true)

			activeStorage := given.Provider().
				WithRandomizer(randomizer.NewTestRandomizer()).
				GORM()

			createActionResult, originalTx := given.Action(activeStorage).Created()

			modifiedTx := *originalTx
			modifiedTx.LockTime = test.lockTime
			for i, seq := range test.sequences {
				if i < len(modifiedTx.Inputs) {
					modifiedTx.Inputs[i].SequenceNumber = seq
				}
			}

			txID := modifiedTx.TxID().String()

			args := wdk.ProcessActionArgs{
				IsNewTx:    true,
				IsSendWith: false,
				IsNoSend:   false,
				IsDelayed:  false,
				Reference:  to.Ptr(createActionResult.Reference),
				TxID:       to.Ptr(primitives.TXIDHexString(txID)),
				RawTx:      modifiedTx.Bytes(),
				SendWith:   []primitives.TXIDHexString{},
			}

			// when:
			result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)
		})
	}
}

func TestProcessActionNLockTimeIsFinalFailure(t *testing.T) {
	tests := map[string]struct {
		setupService func(given testabilities.StorageFixture)
		lockTime     uint32
		sequences    []uint32
	}{
		"block height locktime not final": {
			setupService: func(given testabilities.StorageFixture) {
				const currentHeight = uint32(800_000)
				given.Provider().WhatsOnChain().WillRespondWithChainInfo(http.StatusOK, currentHeight)
			},
			lockTime:  800_001,
			sequences: []uint32{0},
		},
		"timestamp locktime not final": {
			setupService: func(given testabilities.StorageFixture) {
				// No special setup needed for timestamp validation
			},
			lockTime:  uint32(time.Now().Unix() + 7200), //nolint:gosec // unix timestamp fits in uint32
			sequences: []uint32{0},
		},
		"mixed sequences with future block height": {
			setupService: func(given testabilities.StorageFixture) {
				const currentHeight = uint32(500_000)
				given.Provider().WhatsOnChain().WillRespondWithChainInfo(http.StatusOK, currentHeight)
			},
			lockTime:  500_001,
			sequences: []uint32{testutils.MaxSeq - 1, 0, testutils.MaxSeq},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			test.setupService(given)

			activeStorage := given.Provider().
				WithRandomizer(randomizer.NewTestRandomizer()).
				GORM()

			// and:
			createActionResult, originalTx := given.Action(activeStorage).Created()

			modifiedTx := *originalTx
			modifiedTx.LockTime = test.lockTime
			for i, seq := range test.sequences {
				if i < len(modifiedTx.Inputs) {
					modifiedTx.Inputs[i].SequenceNumber = seq
				}
			}

			txID := modifiedTx.TxID().String()

			args := wdk.ProcessActionArgs{
				IsNewTx:    true,
				IsSendWith: false,
				IsNoSend:   false,
				IsDelayed:  false,
				Reference:  to.Ptr(createActionResult.Reference),
				TxID:       to.Ptr(primitives.TXIDHexString(txID)),
				RawTx:      modifiedTx.Bytes(),
				SendWith:   []primitives.TXIDHexString{},
			}

			// when:
			_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

			// then:
			require.Error(t, err)
			require.Contains(t, err.Error(), "transaction nLockTime is not final")
		})
	}
}

func TestProcessActionNLockTimeIsFinalServiceError(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	givenProvider := given.Provider()

	err := givenProvider.WhatsOnChain().WillBeUnreachable()
	require.Error(t, err)
	givenProvider.Bitails().WillReturnNetworkInfo(http.StatusBadGateway, 0)
	err = givenProvider.BHS().WillBeUnreachable()
	require.Error(t, err)

	activeStorage := givenProvider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	createActionResult, originalTx := given.Action(activeStorage).Created()

	modifiedTx := *originalTx
	modifiedTx.LockTime = 400_000
	if len(modifiedTx.Inputs) > 0 {
		modifiedTx.Inputs[0].SequenceNumber = 0
	}

	txID := modifiedTx.TxID().String()

	args := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      modifiedTx.Bytes(),
		SendWith:   []primitives.TXIDHexString{},
	}

	// when:
	_, err = activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to check nLockTime finality")
}

func TestProcessActionNLockTimeIsFinalThresholdBoundary(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// These tests are supposed to check nLockTime and not scripts verification so we mock it here
	given.Provider().ScriptsVerifier().WillReturnBool(true)

	createActionResult, originalTx := given.Action(activeStorage).Created()

	modifiedTx := *originalTx
	modifiedTx.LockTime = nLockTimeThreshold
	if len(modifiedTx.Inputs) > 0 {
		modifiedTx.Inputs[0].SequenceNumber = 0
	}

	txID := modifiedTx.TxID().String()

	args := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      modifiedTx.Bytes(),
		SendWith:   []primitives.TXIDHexString{},
	}

	// when:
	result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)
	require.NotNil(t, result)
}
