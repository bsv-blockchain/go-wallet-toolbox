package actions

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// callRecorder records the order of significant calls so a test can assert relative ordering
// (e.g. the attempts bump happens AFTER the post round, not before).
type callRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *callRecorder) record(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, name)
}

func (r *callRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func (r *callRecorder) count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.calls {
		if c == name {
			n++
		}
	}
	return n
}

// bgSpyKnownTxRepo is the process's knownTxRepo: it records each attempts bump (call + txIDs).
// claimReturns stands in for the claim query's own ordering, which is the database's, not the
// caller's - when set, it is returned instead of the txIDs that were asked for.
func (s *bgSpyKnownTxRepo) ClaimKnownTxsForBroadcast(_ context.Context, txIDs []string) ([]string, error) {
	if s.claimReturns != nil {
		return s.claimReturns, nil
	}
	return txIDs, nil
}

type bgSpyKnownTxRepo struct {
	KnownTxRepo // embed interface; unimplemented methods panic if unexpectedly called

	rec           *callRecorder
	attemptsCalls [][]string
	claimReturns  []string
}

func (s *bgSpyKnownTxRepo) IncreaseKnownTxAttemptsForTxIDs(_ context.Context, txIDs []string) error {
	s.rec.record("bump")
	s.attemptsCalls = append(s.attemptsCalls, txIDs)
	return nil
}

// bgStubServices is the process's services: PostFromBEEF records the post and returns a canned result.
type bgStubServices struct {
	wdk.Services // embed interface; unimplemented methods panic if unexpectedly called

	rec     *callRecorder
	result  wdk.PostFromBeefResult
	postErr error

	postedTxIDs []string
}

func (b *bgStubServices) PostFromBEEF(_ context.Context, _ *transaction.Beef, txIDs []string) (wdk.PostFromBeefResult, error) {
	b.rec.record("post")
	b.postedTxIDs = txIDs
	return b.result, b.postErr
}

// bgStubTxRepo is the process's txRepo: only the reference/label lookups BackgroundBroadcast needs.
type bgStubTxRepo struct {
	TransactionsRepo // embed interface; unimplemented methods panic if unexpectedly called
}

func (bgStubTxRepo) FindReferencesByTxIDs(_ context.Context, _ []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (bgStubTxRepo) GetLabelsForTxIDs(_ context.Context, _ []string) (map[string][]string, error) {
	return map[string][]string{}, nil
}

// --- Unit-of-work providers used inside updateSingleTx (the per-tx apply of a successful round) ---

type bgUowTxRepo struct{ TransactionsRepo }

func (bgUowTxRepo) UpdateTransactionStatusByTxID(_ context.Context, _ string, _ wdk.TxStatus, _ ...wdk.TxStatus) error {
	return nil
}

type bgUowKnownTxRepo struct{ KnownTxRepo }

func (bgUowKnownTxRepo) UpdateKnownTxStatus(_ context.Context, _ string, _ wdk.ProvenTxReqStatus, _ []wdk.ProvenTxReqStatus, _ []history.Builder) error {
	return nil
}

type bgUowUTXORepo struct{ UTXORepo }

func (bgUowUTXORepo) CreateUTXOForSpendableOutputsByTxID(_ context.Context, _ string) error {
	return nil
}

type bgUowProviders struct{ Providers }

func (bgUowProviders) TransactionsRepo() TransactionsRepo { return bgUowTxRepo{} }
func (bgUowProviders) KnownTxRepo() KnownTxRepo           { return bgUowKnownTxRepo{} }
func (bgUowProviders) UTXORepo() UTXORepo                 { return bgUowUTXORepo{} }

type bgUow struct{}

func (bgUow) Do(ctx context.Context, fn func(ctx context.Context, p Providers) error) error {
	return fn(ctx, bgUowProviders{})
}

func newSuccessPostResult(txID string) wdk.PostFromBeefResult {
	return wdk.PostFromBeefResult{
		&wdk.PostFromBEEFServiceResult{
			Name: "ARC",
			PostedBEEFResult: &wdk.PostedBEEF{
				TxIDResults: []wdk.PostedTxID{{Result: wdk.PostedTxIDResultSuccess, TxID: txID}},
			},
		},
	}
}

// TestBackgroundBroadcast_BumpsAttemptsAfterPost pins the attempts-honesty contract for the
// delayed/background path: BackgroundBroadcast increments the attempt counter for the txIDs it
// posts exactly once, AFTER a completed post round (post + per-tx status/UTXO apply) - never before
// the post. Counting after the post keeps the async background worker from touching the shared
// known_tx row early and racing concurrent internalize/abort UTXO accounting on MVCC engines; a
// post that never completes is conservatively NOT counted (ApplyProofTimeouts then fires later).
func TestBackgroundBroadcast_BumpsAttemptsAfterPost(t *testing.T) {
	const txID = "deadbeef"

	t.Run("bumps attempts exactly once after a completed post round", func(t *testing.T) {
		rec := &callRecorder{}
		spy := &bgSpyKnownTxRepo{rec: rec}
		p := &process{
			logger:      logging.NewTestLogger(t),
			knownTxRepo: spy,
			services:    &bgStubServices{rec: rec, result: newSuccessPostResult(txID)},
			txRepo:      bgStubTxRepo{},
			uow:         bgUow{},
		}

		_, err := p.BackgroundBroadcast(context.Background(), nil, []string{txID})
		require.NoError(t, err)

		// The bump ran exactly once, for the posted txIDs...
		require.Len(t, spy.attemptsCalls, 1, "expected exactly one attempts bump per post round")
		assert.Equal(t, []string{txID}, spy.attemptsCalls[0])
		// ...and it ran AFTER the post, not before.
		assert.Equal(t, []string{"post", "bump"}, rec.snapshot())
	})

	t.Run("does not bump when the post itself fails", func(t *testing.T) {
		rec := &callRecorder{}
		spy := &bgSpyKnownTxRepo{rec: rec}
		p := &process{
			logger:      logging.NewTestLogger(t),
			knownTxRepo: spy,
			services:    &bgStubServices{rec: rec, postErr: errors.New("boom: post from beef failed")},
		}

		_, err := p.BackgroundBroadcast(context.Background(), nil, []string{txID})

		// A post that never completed is not counted as an attempt (conservative under-count).
		require.Error(t, err)
		assert.Zero(t, rec.count("bump"), "attempts must not be bumped when the post failed")
		assert.Empty(t, spy.attemptsCalls)
	})
}

// TestBackgroundBroadcast_KeepsTheOrderItWasGiven pins the ordering contract of the re-claim.
// Add() orders an item parents-first, and PostFromBEEF posts the slice in order, so the claim -
// which matches on `tx_id IN (...)` and carries the database's ordering - must be used as a
// filter, never as the new order. Adopting it used to send a child upstream ahead of its parent.
func TestBackgroundBroadcast_KeepsTheOrderItWasGiven(t *testing.T) {
	parent, child, gone := "aaa-parent", "bbb-child", "ccc-aborted"

	t.Run("a reordered claim result does not reorder the post", func(t *testing.T) {
		rec := &callRecorder{}
		services := &bgStubServices{rec: rec, result: newSuccessPostResult(parent)}
		p := &process{
			logger:      logging.NewTestLogger(t),
			knownTxRepo: &bgSpyKnownTxRepo{rec: rec, claimReturns: []string{child, parent}},
			services:    services,
			txRepo:      bgStubTxRepo{},
			uow:         bgUow{},
		}

		_, err := p.BackgroundBroadcast(context.Background(), nil, []string{parent, child})
		require.NoError(t, err)

		assert.Equal(t, []string{parent, child}, services.postedTxIDs)
	})

	t.Run("what the claim rejected is dropped, the rest keeps its order", func(t *testing.T) {
		rec := &callRecorder{}
		services := &bgStubServices{rec: rec, result: newSuccessPostResult(parent)}
		spy := &bgSpyKnownTxRepo{rec: rec, claimReturns: []string{child, parent}}
		p := &process{
			logger:      logging.NewTestLogger(t),
			knownTxRepo: spy,
			services:    services,
			txRepo:      bgStubTxRepo{},
			uow:         bgUow{},
		}

		// gone was aborted while queued: it must not reach the network, and must not drag the
		// others out of order on its way out.
		_, err := p.BackgroundBroadcast(context.Background(), nil, []string{parent, gone, child})
		require.NoError(t, err)

		assert.Equal(t, []string{parent, child}, services.postedTxIDs)
		require.Len(t, spy.attemptsCalls, 1)
		assert.Equal(t, []string{parent, child}, spy.attemptsCalls[0], "attempts are bumped for what is actually posted")
	})
}

// --- Lock-order regression cover for updateSingleTx's unit of work ---

// bgOrderUowKnownTxRepo / bgOrderUowTxRepo record which table is written first and let a test
// drive either guard into ErrStatusUpdateSkipped.
type bgOrderUowKnownTxRepo struct {
	KnownTxRepo

	rec              *callRecorder
	skipStatusesSeen []wdk.ProvenTxReqStatus
	skip             bool
}

func (s *bgOrderUowKnownTxRepo) UpdateKnownTxStatus(_ context.Context, txID string, status wdk.ProvenTxReqStatus, skipForStatuses []wdk.ProvenTxReqStatus, _ []history.Builder) error {
	s.rec.record("known_tx")
	s.skipStatusesSeen = skipForStatuses
	if s.skip {
		return fmt.Errorf("%w: known tx %s -> %s", repo.ErrStatusUpdateSkipped, txID, status)
	}
	return nil
}

type bgOrderUowTxRepo struct {
	TransactionsRepo

	rec  *callRecorder
	skip bool
}

func (s *bgOrderUowTxRepo) UpdateTransactionStatusByTxID(_ context.Context, txID string, status wdk.TxStatus, _ ...wdk.TxStatus) error {
	s.rec.record("transactions")
	if s.skip {
		return fmt.Errorf("%w: transaction(s) with txID %s -> %s", repo.ErrStatusUpdateSkipped, txID, status)
	}
	return nil
}

type bgOrderUowUTXORepo struct {
	UTXORepo

	rec *callRecorder
}

func (s *bgOrderUowUTXORepo) CreateUTXOForSpendableOutputsByTxID(_ context.Context, _ string) error {
	s.rec.record("utxo")
	return nil
}

type bgOrderUowProviders struct {
	Providers

	knownTx *bgOrderUowKnownTxRepo
	txs     *bgOrderUowTxRepo
	utxo    *bgOrderUowUTXORepo
}

func (p bgOrderUowProviders) TransactionsRepo() TransactionsRepo { return p.txs }
func (p bgOrderUowProviders) KnownTxRepo() KnownTxRepo           { return p.knownTx }
func (p bgOrderUowProviders) UTXORepo() UTXORepo                 { return p.utxo }

type bgOrderUow struct{ providers bgOrderUowProviders }

func (u bgOrderUow) Do(ctx context.Context, fn func(ctx context.Context, p Providers) error) error {
	return fn(ctx, u.providers)
}

// TestUpdateSingleTx_WritesKnownTxBeforeTransactions pins the lock order of the post-broadcast
// apply - shared KnownTx before the per-user Transaction rows - and the guard semantics that
// ride on it. The reverse order deadlocked against the status-sync cron.
func TestUpdateSingleTx_WritesKnownTxBeforeTransactions(t *testing.T) {
	const txID = "deadbeef"

	newProcess := func(knownTx *bgOrderUowKnownTxRepo, txs *bgOrderUowTxRepo, utxo *bgOrderUowUTXORepo) *process {
		return &process{
			logger: logging.NewTestLogger(t),
			uow:    bgOrderUow{providers: bgOrderUowProviders{knownTx: knownTx, txs: txs, utxo: utxo}},
		}
	}

	successResult := &wdk.AggregatedPostedTxID{Status: wdk.AggregatedPostedTxIDSuccess}

	t.Run("known_tx is written before transactions", func(t *testing.T) {
		rec := &callRecorder{}
		knownTx := &bgOrderUowKnownTxRepo{rec: rec}
		p := newProcess(knownTx, &bgOrderUowTxRepo{rec: rec}, &bgOrderUowUTXORepo{rec: rec})

		_, _, err := p.updateSingleTx(context.Background(), txID, successResult, nil, nil, []string{txID}, "", nil)
		require.NoError(t, err)

		assert.Equal(t, []string{"known_tx", "transactions", "utxo"}, rec.snapshot())
	})

	t.Run("the known_tx guard covers terminal failures, not just the beyond-broadcast statuses", func(t *testing.T) {
		rec := &callRecorder{}
		knownTx := &bgOrderUowKnownTxRepo{rec: rec}
		p := newProcess(knownTx, &bgOrderUowTxRepo{rec: rec}, &bgOrderUowUTXORepo{rec: rec})

		_, _, err := p.updateSingleTx(context.Background(), txID, successResult, nil, nil, []string{txID}, "", nil)
		require.NoError(t, err)

		assert.Subset(t, knownTx.skipStatusesSeen, wdk.ProvenTxReqBeyondBroadcastStageStatuses)
		assert.Contains(t, knownTx.skipStatusesSeen, wdk.ProvenTxStatusInvalid)
		assert.Contains(t, knownTx.skipStatusesSeen, wdk.ProvenTxStatusDoubleSpend)
	})

	t.Run("a skipped known_tx guard makes the whole unit of work a no-op", func(t *testing.T) {
		rec := &callRecorder{}
		p := newProcess(&bgOrderUowKnownTxRepo{rec: rec, skip: true}, &bgOrderUowTxRepo{rec: rec}, &bgOrderUowUTXORepo{rec: rec})

		_, _, err := p.updateSingleTx(context.Background(), txID, successResult, nil, nil, []string{txID}, "", nil)

		require.NoError(t, err)
		assert.Equal(t, []string{"known_tx"}, rec.snapshot())
	})

	t.Run("a skipped transaction CAS keeps the known_tx transition and skips the rest", func(t *testing.T) {
		rec := &callRecorder{}
		p := newProcess(&bgOrderUowKnownTxRepo{rec: rec}, &bgOrderUowTxRepo{rec: rec, skip: true}, &bgOrderUowUTXORepo{rec: rec})

		_, _, err := p.updateSingleTx(context.Background(), txID, successResult, nil, nil, []string{txID}, "", nil)

		require.NoError(t, err)
		assert.Equal(t, []string{"known_tx", "transactions"}, rec.snapshot())
	})
}

// TestRetainClaimed pins the filter both broadcast paths run the claim result through.
// ClaimKnownTxsForBroadcast matches on `tx_id IN (...)` with no ORDER BY, so its result carries
// the database's ordering - taking it as the new order is what used to send a child upstream
// ahead of its parent.
func TestRetainClaimed(t *testing.T) {
	oldest, middle, newest := "aaa-oldest", "mmm-middle", "zzz-newest"

	t.Run("keeps the caller's order when everything was claimed", func(t *testing.T) {
		// The claim came back sorted by txid; the caller asked in creation order.
		assert.Equal(t,
			[]string{newest, oldest, middle},
			retainClaimed([]string{newest, oldest, middle}, []string{oldest, middle, newest}),
		)
	})

	t.Run("drops what was not claimed and keeps the rest in order", func(t *testing.T) {
		assert.Equal(t,
			[]string{newest, middle},
			retainClaimed([]string{newest, oldest, middle}, []string{middle, newest}),
		)
	})

	t.Run("an empty claim retains nothing", func(t *testing.T) {
		assert.Empty(t, retainClaimed([]string{oldest, middle}, nil))
	})

	t.Run("does not modify the input slice", func(t *testing.T) {
		input := []string{newest, oldest, middle}
		retainClaimed(input, []string{oldest})
		assert.Equal(t, []string{newest, oldest, middle}, input)
	})
}
