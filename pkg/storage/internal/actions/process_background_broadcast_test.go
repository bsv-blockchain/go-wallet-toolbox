package actions

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
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
func (s *bgSpyKnownTxRepo) ClaimKnownTxsForBroadcast(_ context.Context, txIDs []string) ([]string, error) {
	return txIDs, nil
}

type bgSpyKnownTxRepo struct {
	KnownTxRepo // embed interface; unimplemented methods panic if unexpectedly called

	rec           *callRecorder
	attemptsCalls [][]string
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
}

func (b *bgStubServices) PostFromBEEF(_ context.Context, _ *transaction.Beef, _ []string) (wdk.PostFromBeefResult, error) {
	b.rec.record("post")
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
