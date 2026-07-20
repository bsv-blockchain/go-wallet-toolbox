package storage_test

import (
	"context"
	"testing"
	"time"

	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	pkgtestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestInternalizeAction_UpdateKnownTxAsMined_HappyPath(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	provider := given.Provider()
	activeStorage := provider.GORM()
	whatsOnChain := provider.WhatsOnChain()

	// and:
	tx := whatsOnChain.MinedTransaction().Tx()
	txID := tx.TxID()

	root, err := tx.MerklePath.ComputeRoot(txID)
	require.NoError(t, err)
	require.NotNil(t, root)

	atomicBEEF, err := tx.AtomicBEEF(false)
	require.NoError(t, err)
	require.NotNil(t, atomicBEEF)

	// and:
	whatsOnChain.WillRespondWithMerkleRoot(root.String())

	args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
	args.Tx = atomicBEEF

	// and:
	expectedResult := &wdk.InternalizeActionResult{
		Accepted: true,
		IsMerge:  false,
		TxID:     txID.String(),
		Satoshis: 2324,
	}

	// when:
	actualResult, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	require.Equal(t, expectedResult, actualResult)
	// Mined txs are not re-broadcast: no send/review result fields.
	assert.Empty(t, actualResult.SendWithResults)
	assert.Empty(t, actualResult.NotDelayedResults)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID.String()).WithBlockHash(to.Ptr(pkgtestabilities.TestBlockHash))
}

func TestInternalizeActionNilAuth(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.InternalizeAction(t.Context(), wdk.AuthID{UserID: nil}, fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol))

	// then:
	require.Error(t, err)
}

func TestInternalizeActionWalletPaymentHappyPath(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)

	// when:
	result, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)

	assert.True(t, result.Accepted)
	assert.False(t, result.IsMerge)
	assert.Equal(t, int64(fixtures.ExpectedValueToInternalize), result.Satoshis)
	assert.Equal(t, "03895fb984362a4196bc9931629318fcbb2aeba7c6293638119ea653fa31d119", result.TxID)

	// Immediate broadcast via ProcessAction path surfaces send/review results (TS shareReqsWithWorld).
	require.Len(t, result.SendWithResults, 1)
	assert.Equal(t, result.TxID, string(result.SendWithResults[0].TxID))
	assert.Equal(t, wdk.SendWithResultStatusUnproven, result.SendWithResults[0].Status)
	require.Len(t, result.NotDelayedResults, 1)
	assert.Equal(t, result.TxID, string(result.NotDelayedResults[0].TxID))
	assert.Equal(t, wdk.ReviewActionResultStatusSuccess, result.NotDelayedResults[0].Status)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(result.TxID).
		NotMined().
		WithStatus(wdk.ProvenTxStatusUnmined).
		WithAttempts(1).
		TxNotes(func(then testabilities.TxNotesAssertion) {
			then.
				Count(4).
				Note("internalizeAction", to.Ptr(testusers.Alice.ID), nil).
				Note("postBeefSuccess", nil, nil).
				Note("postBeefError", nil, nil).
				Note("aggregateResults", nil, nil)
		})

	thenDBState.AllOutputs(testusers.Alice).WithCountHavingTxID(1)
}

func TestInternalizeActionBasketInsertionHappyPath(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	args := fixtures.DefaultInternalizeActionArgs(t, wdk.BasketInsertionProtocol)

	// when:
	result, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)

	assert.True(t, result.Accepted)
	assert.False(t, result.IsMerge)
	assert.Equal(t, int64(0), result.Satoshis)
	assert.Equal(t, "03895fb984362a4196bc9931629318fcbb2aeba7c6293638119ea653fa31d119", result.TxID)

	require.Len(t, result.SendWithResults, 1)
	assert.Equal(t, wdk.SendWithResultStatusUnproven, result.SendWithResults[0].Status)
	require.Len(t, result.NotDelayedResults, 1)
	assert.Equal(t, wdk.ReviewActionResultStatusSuccess, result.NotDelayedResults[0].Status)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(result.TxID).
		NotMined().
		WithStatus(wdk.ProvenTxStatusUnmined).
		WithAttempts(1).
		TxNotes(func(then testabilities.TxNotesAssertion) {
			then.
				Count(4).
				Note("internalizeAction", to.Ptr(testusers.Alice.ID), nil).
				Note("postBeefSuccess", nil, nil).
				Note("postBeefError", nil, nil).
				Note("aggregateResults", nil, nil)
		})

	thenDBState.Outputs(testusers.Alice, wdk.BasketNameForChange).WithCount(0)
	thenDBState.Outputs(testusers.Alice, fixtures.CustomBasket).WithCountHavingTxID(1)
}

func TestInternalizeActionErrorCases(t *testing.T) {
	tests := map[string]struct {
		modifier func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs
	}{
		"Wrong beef": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				args.Tx = []byte{0, 1, 2, 3}
				return args
			},
		},
		"Output index out of range of provided tx": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				args.Outputs[0].OutputIndex = fixtures.ExpectedValueToInternalize
				return args
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			// given:
			activeStorage := given.Provider().GORM()

			// and:
			args := test.modifier(fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol))

			// when:
			_, err := activeStorage.InternalizeAction(
				t.Context(),
				testusers.Alice.AuthID(),
				args,
			)

			// then:
			require.Error(t, err)

			// and db state:
			thenDBState := testabilities.ThenDBState(t, activeStorage)
			thenDBState.AllOutputs(testusers.Alice).WithCount(0)
		})
	}
}

func TestInternalizeActionForAlreadyStoredTransaction(t *testing.T) {
	t.Run("the same output", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		activeStorage := given.Provider().GORM()

		// and:
		const alreadyOwnedSatoshis = 100_000
		ownedTxSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(alreadyOwnedSatoshis)

		// and:
		args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
		args.Tx = ownedTxSpec.AtomicBEEF().Bytes()

		// when:
		result, err := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
		assert.Equal(t, ownedTxSpec.ID().String(), result.TxID)
		assert.True(t, result.Accepted)
		assert.True(t, result.IsMerge)
		assert.Equal(t, int64(0), result.Satoshis)

		// and db state:
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasKnownTX(result.TxID).
			NotMined().
			WithStatus(wdk.ProvenTxStatusUnmined)

		thenDBState.AllOutputs(testusers.Alice).WithCount(1)
		thenDBState.Outputs(testusers.Alice, wdk.BasketNameForChange).WithCountHavingTxID(1)
	})

	t.Run("two outputs - two basket insertions", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		activeStorage := given.Provider().GORM()

		// and:
		transactionSpec := testvectors.GivenTX().
			WithInput(20_001).
			WithP2PKHOutput(10_000).
			WithP2PKHOutput(10_000)

		// when:
		result, err := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			wdk.InternalizeActionArgs{
				Tx: transactionSpec.AtomicBEEF().Bytes(),
				Outputs: []*wdk.InternalizeOutput{
					{
						OutputIndex: 0,
						Protocol:    wdk.BasketInsertionProtocol,
						InsertionRemittance: &wdk.BasketInsertion{
							Basket: fixtures.CustomBasket,
							Tags:   []primitives.StringUnder300{"custom_tag", "tag_for_first_output"},
						},
					},
				},
				Description: "first internalize",
			},
		)

		// then:
		require.NoError(t, err)
		assert.True(t, result.Accepted)
		assert.False(t, result.IsMerge)
		assert.Equal(t, int64(0), result.Satoshis)

		// when:
		result, err = activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			wdk.InternalizeActionArgs{
				Tx: transactionSpec.AtomicBEEF().Bytes(),
				Outputs: []*wdk.InternalizeOutput{
					{
						OutputIndex: 1,
						Protocol:    wdk.BasketInsertionProtocol,
						InsertionRemittance: &wdk.BasketInsertion{
							Basket: fixtures.CustomBasket,
							Tags:   []primitives.StringUnder300{"custom_tag", "tag_for_second_output"},
						},
					},
				},
				Description: "second internalize",
			},
		)

		// then:
		require.NoError(t, err)
		assert.True(t, result.Accepted)
		assert.True(t, result.IsMerge)
		assert.Equal(t, int64(0), result.Satoshis)
		// and db state:
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasKnownTX(result.TxID).
			NotMined().
			WithStatus(wdk.ProvenTxStatusUnmined)

		thenDBState.AllOutputs(testusers.Alice).WithCount(2)
		thenDBState.Outputs(testusers.Alice, wdk.BasketNameForChange).WithCount(0)
		thenDBState.Outputs(testusers.Alice, fixtures.CustomBasket).
			WithCountHavingTxID(2).
			WithCountHavingTags(2, "custom_tag").
			WithCountHavingTags(1, "tag_for_first_output").
			WithCountHavingTags(1, "tag_for_second_output")
	})

	t.Run("switch from change output to custom basket", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		activeStorage := given.Provider().GORM()

		// and:
		const alreadyOwnedSatoshis = 100_000
		ownedTxSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(alreadyOwnedSatoshis)

		// and:
		args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
		args.Tx = ownedTxSpec.AtomicBEEF().Bytes()
		args.Outputs[0].Protocol = wdk.BasketInsertionProtocol
		args.Outputs[0].InsertionRemittance = &wdk.BasketInsertion{
			Basket: fixtures.CustomBasket,
			Tags:   []primitives.StringUnder300{"custom_tag", "tag_for_first_output"},
		}

		// when:
		result, err := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
		assert.Equal(t, ownedTxSpec.ID().String(), result.TxID)
		assert.True(t, result.Accepted)
		assert.True(t, result.IsMerge)
		assert.Equal(t, int64(-alreadyOwnedSatoshis), result.Satoshis)

		// and db state:
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasKnownTX(result.TxID).
			NotMined().
			WithStatus(wdk.ProvenTxStatusUnmined)

		thenDBState.AllOutputs(testusers.Alice).WithCount(1)
		thenDBState.Outputs(testusers.Alice, wdk.BasketNameForChange).WithCount(0)
		thenDBState.Outputs(testusers.Alice, fixtures.CustomBasket).
			WithCountHavingTxID(1).
			WithCountHavingTags(1, "custom_tag", "tag_for_first_output")
	})

	t.Run("switch from custom basket to change", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		activeStorage := given.Provider().GORM()

		// and:
		internalizeArgs, _ := given.Action(activeStorage).PreInternalized()
		walletPaymentOutput := *internalizeArgs.Outputs[0]

		internalizeArgs.Description = "first internalize"
		internalizeArgs.Outputs = []*wdk.InternalizeOutput{
			{
				OutputIndex: 0,
				Protocol:    wdk.BasketInsertionProtocol,
				InsertionRemittance: &wdk.BasketInsertion{
					Basket: "custom_basket",
					Tags:   []primitives.StringUnder300{"custom_tag"},
				},
			},
		}

		// when:
		result, err := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			*internalizeArgs,
		)

		// then:
		require.NoError(t, err)
		assert.True(t, result.Accepted)
		assert.False(t, result.IsMerge)
		assert.Equal(t, int64(0), result.Satoshis)

		// when:
		internalizeArgs.Description = "second internalize"
		internalizeArgs.Outputs = []*wdk.InternalizeOutput{&walletPaymentOutput}
		result, err = activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			*internalizeArgs,
		)

		// then:
		require.NoError(t, err)
		assert.True(t, result.Accepted)
		assert.True(t, result.IsMerge)
		assert.Equal(t, int64(fixtures.DefaultCreateActionOutputSatoshis), result.Satoshis)

		// and db state:
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasKnownTX(result.TxID).
			NotMined().
			WithStatus(wdk.ProvenTxStatusUnmined)

		thenDBState.AllOutputs(testusers.Alice).WithCount(1)
		thenDBState.Outputs(testusers.Alice, wdk.BasketNameForChange).WithCountHavingTxID(1)
		thenDBState.Outputs(testusers.Alice, fixtures.CustomBasket).WithCount(0)
	})

	t.Run("add label during withMerge internalize", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		activeStorage := given.Provider().GORM()

		// and:
		const (
			alreadyOwnedSatoshis = 100_000
			initialLabel         = "initial_label"
			labelToAdd           = "label_for_merge"
		)
		ownedTxSpec, _ := given.Faucet(activeStorage, testusers.Alice).
			TopUp(alreadyOwnedSatoshis, pkgtestabilities.WithLabelsTopUp(initialLabel))

		// and:
		args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
		args.Tx = ownedTxSpec.AtomicBEEF().Bytes()
		args.Labels = []primitives.StringUnder300{labelToAdd}

		// when:
		_, err := activeStorage.InternalizeAction(
			context.Background(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)

		// and db state:
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasUserTransactionByReference(testusers.Alice, fixtures.FaucetReference(ownedTxSpec.ID().String())).
			WithLabels(initialLabel, labelToAdd)
	})
}

func TestInternalizeTheSameTxByDifferentUsers(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	transactionSpec := testvectors.GivenTX().
		WithInput(20_001).
		WithP2PKHOutput(10_000).
		WithP2PKHOutput(10_000)

	// when:
	result, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.InternalizeActionArgs{
			Tx: transactionSpec.AtomicBEEF().Bytes(),
			Outputs: []*wdk.InternalizeOutput{
				{
					OutputIndex: 0,
					Protocol:    wdk.BasketInsertionProtocol,
					InsertionRemittance: &wdk.BasketInsertion{
						Basket: fixtures.CustomBasket,
					},
				},
			},
			Description: "first internalize",
		},
	)

	// then:
	require.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.False(t, result.IsMerge)
	assert.Equal(t, int64(0), result.Satoshis)

	// when:
	result, err = activeStorage.InternalizeAction(
		t.Context(),
		testusers.Bob.AuthID(), // NOTE: This is a different user
		wdk.InternalizeActionArgs{
			Tx: transactionSpec.AtomicBEEF().Bytes(),
			Outputs: []*wdk.InternalizeOutput{
				{
					OutputIndex: 1,
					Protocol:    wdk.BasketInsertionProtocol,
					InsertionRemittance: &wdk.BasketInsertion{
						Basket: fixtures.CustomBasket,
					},
				},
			},
			Description: "second internalize",
		},
	)

	// then:
	require.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.False(t, result.IsMerge)
	assert.Equal(t, int64(0), result.Satoshis)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(result.TxID).
		NotMined().
		WithStatus(wdk.ProvenTxStatusUnmined)

	thenDBState.AllOutputs(testusers.Alice).WithCount(1)
	thenDBState.AllOutputs(testusers.Bob).WithCount(1)
}

// TestInternalizeAction_ReorgedKnownTx_DoesNotClaimNetworkAcceptance pins the settled
// reorg semantics of W1-6: after a reorg invalidates a tx's proof (KnownTx status=reorg),
// internalizing that same tx for a different user must NOT pretend the network still
// accepts it. Because AlreadySent(reorg)=false now, storeNewTx takes the fresh-tx branch
// (re-queued for broadcast) rather than flipping the KnownTx to unmined-without-evidence.
//
// Internalize now broadcasts synchronously via ProcessAction's path. ARC POST is held so
// InternalizeAction blocks after storeNewTx commits, letting us assert the intermediate
// unsent/sending state before the broadcast mutates KnownTx.
func TestInternalizeAction_ReorgedKnownTx_DoesNotClaimNetworkAcceptance(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and: a mined tx owned by Alice (proof fetched via sync -> status completed, mined)
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	txID := txSpec.ID().String()

	givenProvider.ARC().WhenQueryingTx(txID).WillReturnWithMindedTx()
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})
	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined()

	// and: the block is orphaned by a reorg -> proof invalidated, status reorg, was_broadcast kept.
	err = activeStorage.HandleReorg(t.Context(), []string{testservices.TestBlockHash})
	require.NoError(t, err)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusReorg).
		NotMined().
		WasBroadcast(true)

	// Hold ARC POST so storeNewTx's unsent write is observable before broadcast applies results.
	// Ensure release on all exit paths (success, failure, timeout) — but only once.
	givenProvider.ARC().HoldBroadcasting()
	released := false
	releaseARC := func() {
		if !released {
			released = true
			givenProvider.ARC().ReleaseBroadcasting()
		}
	}
	defer releaseARC()

	type internalizeOutcome struct {
		result *wdk.InternalizeActionResult
		err    error
	}
	done := make(chan internalizeOutcome, 1)

	// when: a DIFFERENT user internalizes the same tx (fresh entry, not a merge).
	// Runs in a goroutine because broadcast is now synchronous and will block on the ARC hold.
	go func() {
		result, internalizeErr := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Bob.AuthID(), // NOTE: different user -> storeNewTx path, not merge
			wdk.InternalizeActionArgs{
				Tx: txSpec.AtomicBEEF().Bytes(),
				Outputs: []*wdk.InternalizeOutput{
					{
						OutputIndex: 0,
						Protocol:    wdk.BasketInsertionProtocol,
						InsertionRemittance: &wdk.BasketInsertion{
							Basket: fixtures.CustomBasket,
						},
					},
				},
				Description: "internalize reorged tx",
			},
		)
		done <- internalizeOutcome{result: result, err: internalizeErr}
	}()

	// Wait until storeNewTx has committed the fresh-send state (KnownTx=unsent or submitting/sending).
	require.Eventually(t, func() bool {
		found, findErr := activeStorage.KnownTxEntity().Read().TxID(txID).Find(t.Context())
		if findErr != nil || len(found) == 0 {
			return false
		}
		status := found[0].Status
		return status == wdk.ProvenTxStatusUnsent || status == wdk.ProvenTxStatusSending
	}, 5*time.Second, 20*time.Millisecond)

	// and db state: the shared KnownTx is NOT flipped to unmined-without-evidence.
	//
	// storeNewTx else-branch (internalize.go): the internalized BEEF carries no merkle proof
	// (isMined=false) and AlreadySent(reorg)=false, so knownTxStatus=unsent with
	// skipForStatuses={completed, unmined, sending, unsent}. reorg is not in that skip list,
	// so upsertKnownTx rewrites the row to unsent (re-queued for broadcast). It stays NotMined
	// and keeps was_broadcast=true so proof re-sync remains eligible.
	//
	// Status may already be "sending" if MarkKnownTxsAsSubmitting ran before we observe.
	found, findErr := activeStorage.KnownTxEntity().Read().TxID(txID).Find(t.Context())
	require.NoError(t, findErr)
	require.NotEmpty(t, found)
	assert.Contains(t, []wdk.ProvenTxReqStatus{wdk.ProvenTxStatusUnsent, wdk.ProvenTxStatusSending}, found[0].Status)
	thenDBState.HasKnownTX(txID).
		NotMined().
		WasBroadcast(true)

	// and: the internalizing user's transaction is in the fresh-send state (sending), not
	// unproven — direct evidence the fresh-tx branch was taken rather than the accepted-unproven one.
	thenDBState.HasUserTransactionsByTxIDsWithStatus(testusers.Bob, wdk.TxStatusSending, txID)

	// and: the internalized output landed for Bob.
	thenDBState.AllOutputs(testusers.Bob).WithCountHavingTxID(1)

	// Release broadcast and wait for InternalizeAction to finish.
	releaseARC()
	var outcome internalizeOutcome
	select {
	case outcome = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for InternalizeAction to complete after releasing ARC hold")
	}

	// then: internalize is consistent (accepted, fresh entry).
	require.NoError(t, outcome.err)
	require.NotNil(t, outcome.result)
	assert.True(t, outcome.result.Accepted)
	assert.False(t, outcome.result.IsMerge)
	assert.Equal(t, txID, outcome.result.TxID)
	// Broadcast was attempted after store — result fields present when soft-success.
	require.NotEmpty(t, outcome.result.SendWithResults)

	// NOTE (concern, out of W1-6 scope): a full AssertStorageInvariants is intentionally NOT
	// gated here. HandleReorg nulls the KnownTx proof but leaves Alice's original user
	// transaction at status=completed, which violates money-safety invariant #2 ("no completed
	// user transaction without a merkle proof"). That gap is pre-existing in HandleReorg and is
	// independent of W1-6 (it reproduces right after HandleReorg, before this internalize). It
	// is reported as a concern, not fixed in this task.
}

// TestInternalizeAction_BroadcastServiceError_SurfacesResultFields pins issue #818:
// when a new unproven tx is internalized and the immediate broadcast returns a soft service
// error, InternalizeAction remains accepted and exposes sendWithResults / notDelayedResults
// so callers can observe broadcast status (matching TS shareReqsWithWorld).
func TestInternalizeAction_BroadcastServiceError_SurfacesResultFields(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()
	args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
	txID := "03895fb984362a4196bc9931629318fcbb2aeba7c6293638119ea653fa31d119"

	// and: ARC cannot confirm anything about this tx (empty response body) — bare service error.
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnNoBody()

	// when:
	result, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then: internalize is still accepted; broadcast outcome is surfaced for the caller.
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Accepted)
	assert.False(t, result.IsMerge)
	assert.Equal(t, txID, result.TxID)

	require.Len(t, result.SendWithResults, 1)
	assert.Equal(t, txID, string(result.SendWithResults[0].TxID))
	// Service error keeps the send in-flight (sending), not terminal failure.
	assert.Equal(t, wdk.SendWithResultStatusSending, result.SendWithResults[0].Status)

	require.Len(t, result.NotDelayedResults, 1)
	assert.Equal(t, txID, string(result.NotDelayedResults[0].TxID))
	assert.Equal(t, wdk.ReviewActionResultStatusServiceError, result.NotDelayedResults[0].Status)

	// and db: KnownTx stays retryable (sending), matching ProcessAction service-error semantics.
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusSending)
	thenDBState.HasUserTransactionByTxID(testusers.Alice, txID).WithStatus(wdk.TxStatusSending)
}

// TestInternalizeAction_MergePath_OmitsBroadcastResultFields ensures merge internalizes do not
// re-broadcast or populate send/review result fields.
func TestInternalizeAction_MergePath_OmitsBroadcastResultFields(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()
	const alreadyOwnedSatoshis = 100_000
	ownedTxSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(alreadyOwnedSatoshis)

	args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
	args.Tx = ownedTxSpec.AtomicBEEF().Bytes()

	// when:
	result, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.True(t, result.IsMerge)
	assert.Empty(t, result.SendWithResults)
	assert.Empty(t, result.NotDelayedResults)
}
