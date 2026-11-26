package storage_test

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TestGroupCommit_ConcurrentActionsWithMixedOutcomes runs concurrent createAction calls - some
// fundable, some not - and then concurrent processAction calls for the funded ones, with group
// commit on. Failed units must not affect their batch-mates, and every phase must have shared
// transactions.
func TestGroupCommit_ConcurrentActionsWithMixedOutcomes(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	t.Cleanup(func() { requireNoCommitterGoroutines(t) })
	t.Cleanup(cleanup)

	// One desired change UTXO: the funder then takes nothing beyond each action's own input, so the
	// fundable actions never compete for each other's parents (the privacy heuristics would make
	// them, grouped or not).
	activeStorage := given.Provider().
		WithChangeBasket(defs.ChangeBasket{NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1, MaxChangeOutputsPerTx: 1}).
		WithGroupCommit(defs.GroupCommit{Enabled: true, MaxBatch: 8, MaxWait: 200 * time.Millisecond}).
		GORM()

	const (
		fundable   = 4
		unfundable = 2
	)

	// and: every action spends its own internalized parent; the unfundable ones also ask for far
	// more than the wallet holds.
	action := given.Action(activeStorage)
	args := make([]wdk.ValidCreateActionArgs, 0, fundable+unfundable)
	for i := range fundable + unfundable {
		_, parent := action.WithSatoshisToInternalize(uint64(40_000 + i)).Internalized()
		a := action.CreateActionArgsSpending(parent)
		if i >= fundable {
			a.Outputs[0].Satoshis = primitives.SatoshiValue(1_000_000_000)
		}
		args = append(args, a)
	}

	// internalizing goes through the grouped broadcast-result path too; count from here
	batchesBefore, unitsBefore := storage.GroupCommitStats(activeStorage)

	// when: all createAction calls run at once
	results := make([]*wdk.StorageCreateActionResult, len(args))
	errs := make([]error, len(args))
	runConcurrently(len(args), func(i int) {
		results[i], errs[i] = activeStorage.CreateAction(context.Background(), testusers.Alice.AuthID(), args[i])
	})

	// then:
	for i := range args {
		if i < fundable {
			require.NoError(t, errs[i], "action %d", i)
			continue
		}
		require.ErrorIs(t, errs[i], wdk.ErrNotEnoughFunds, "action %d", i)
	}

	// when: the funded actions are signed and processed at once
	processArgs := make([]wdk.ProcessActionArgs, fundable)
	for i := range fundable {
		signed := action.Sign(results[i])
		processArgs[i] = wdk.ProcessActionArgs{
			IsNewTx:   true,
			Reference: to.Ptr(results[i].Reference),
			TxID:      to.Ptr(primitives.TXIDHexString(signed.TxID().String())),
			RawTx:     signed.Bytes(),
		}
	}
	processResults := make([]*wdk.ProcessActionResult, fundable)
	runConcurrently(fundable, func(i int) {
		processResults[i], errs[i] = activeStorage.ProcessAction(context.Background(), testusers.Alice.AuthID(), processArgs[i])
	})

	// then: every action was broadcast, claimed once (attempts = 1) and moved to unproven
	for i := range fundable {
		require.NoError(t, errs[i], "process %d", i)
		require.Len(t, processResults[i].NotDelayedResults, 1)
		assert.Equal(t, wdk.ReviewActionResultStatusSuccess, processResults[i].NotDelayedResults[0].Status)

		txID := string(*processArgs[i].TxID)
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined).WithAttempts(1)
		thenDBState.HasUserTransactionByReference(testusers.Alice, results[i].Reference).WithStatus(wdk.TxStatusUnproven)
	}

	// and: the phases ran through the committer and shared transactions. 6 funding units, then
	// spend, claim and broadcast result for each of the 4 processed actions. A unit the engine
	// picked as a deadlock victim (Postgres) runs on its own instead and is not counted.
	batches, units := storage.GroupCommitStats(activeStorage)
	batches, units = batches-batchesBefore, units-unitsBefore
	assert.LessOrEqual(t, units, uint64(fundable+unfundable+3*fundable))
	assert.Positive(t, units)
	assert.Less(t, batches, units, "concurrent units must share transactions")
}

// TestGroupCommit_ResultsMatchUngroupedPath runs the same sequence of actions with group commit
// off and on, with a deterministic randomizer, and expects identical results and errors.
func TestGroupCommit_ResultsMatchUngroupedPath(t *testing.T) {
	type outcome struct {
		created       []*wdk.StorageCreateActionResult
		notEnoughErr  string
		canceledErr   string
		knownTxStatus []wdk.ProvenTxReqStatus
	}

	run := func(t *testing.T, groupCommit defs.GroupCommit) outcome {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := given.Provider().
			WithRandomizer(randomizer.NewTestRandomizer()).
			WithGroupCommit(groupCommit).
			GORM()

		var out outcome
		action := given.Action(activeStorage)
		for i := range 2 {
			// a different amount gives each action its own parent transaction
			created, signed := action.WithSatoshisToInternalize(uint64(40_000 + i)).Processed()
			out.created = append(out.created, created)
			out.knownTxStatus = append(out.knownTxStatus, knownTxStatus(t, activeStorage, signed.TxID().String()))
		}

		_, parent := action.WithSatoshisToInternalize(40_002).Internalized()
		args := action.CreateActionArgsSpending(parent)
		args.Outputs[0].Satoshis = primitives.SatoshiValue(1_000_000_000)
		_, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), args)
		require.Error(t, err)
		out.notEnoughErr = err.Error()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		args.Outputs[0].Satoshis = 1
		_, err = activeStorage.CreateAction(ctx, testusers.Alice.AuthID(), args)
		require.Error(t, err)
		out.canceledErr = err.Error()

		return out
	}

	off := run(t, defs.GroupCommit{})
	on := run(t, defs.GroupCommit{Enabled: true})

	assert.Equal(t, off.created, on.created)
	assert.Equal(t, off.knownTxStatus, on.knownTxStatus)
	assert.Equal(t, off.notEnoughErr, on.notEnoughErr)
	assert.Equal(t, off.canceledErr, on.canceledErr)
}

func knownTxStatus(t *testing.T, provider *storage.Provider, txID string) wdk.ProvenTxReqStatus {
	t.Helper()
	statuses, err := provider.Database.CreateRepositories().FindKnownTxStatuses(t.Context(), txID)
	require.NoError(t, err)
	return statuses[txID]
}

// runConcurrently runs fn(0..n-1) in n goroutines released together.
func runConcurrently(n int, fn func(i int)) {
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			fn(i)
		}()
	}
	close(start)
	wg.Wait()
}

// requireNoCommitterGoroutines fails when a group committer goroutine survived the provider's Stop.
func requireNoCommitterGoroutines(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		buf := make([]byte, 1<<20)
		buf = buf[:runtime.Stack(buf, true)]
		return !strings.Contains(string(buf), "groupcommit.(*Committer")
	}, 5*time.Second, 10*time.Millisecond, "group commit goroutines must exit when the provider stops")
}
