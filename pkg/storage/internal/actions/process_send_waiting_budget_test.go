package actions

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// swSpyKnownTxRepo serves the two calls the sweep makes before it reaches anything
// heavier: the page query that discovers waiting transactions, and the status
// lookup that opens broadcastTxs.
type swSpyKnownTxRepo struct {
	KnownTxRepo // embed interface; unimplemented methods panic if unexpectedly called

	pages     [][]*entity.KnownTxForStatusSync
	pageErrs  []error
	pageCalls int

	statusErr    error
	statusCalls  int
	statusesSeen [][]string
}

func (s *swSpyKnownTxRepo) FindKnownTxIDsByStatuses(_ context.Context, _ []wdk.ProvenTxReqStatus, _ ...queryopts.Options) ([]*entity.KnownTxForStatusSync, error) {
	i := s.pageCalls
	s.pageCalls++
	if i < len(s.pageErrs) && s.pageErrs[i] != nil {
		return nil, s.pageErrs[i]
	}
	if i < len(s.pages) {
		return s.pages[i], nil
	}
	return nil, nil
}

func (s *swSpyKnownTxRepo) FindKnownTxStatuses(_ context.Context, txIDs ...string) (map[string]wdk.ProvenTxReqStatus, error) {
	s.statusCalls++
	s.statusesSeen = append(s.statusesSeen, txIDs)
	return nil, s.statusErr
}

func waitingTx(txID string, batch *string) *entity.KnownTxForStatusSync {
	return &entity.KnownTxForStatusSync{TxID: txID, Status: wdk.ProvenTxStatusUnsent, Batch: batch}
}

func newSweepProcess(t *testing.T, repo *swSpyKnownTxRepo) *process {
	t.Helper()
	return &process{logger: logging.NewTestLogger(t), knownTxRepo: repo}
}

// TestSendWaitingTransactions_StopsBeforeStartingABatchItCannotFinish reproduces the
// incident this guard exists for: the monitor budgets the whole run, and a run that
// starts moments before its own scheduled instant gets a budget of milliseconds. The
// sweep must then defer everything as one summary rather than let every batch fail on
// its first query and emit an error line apiece.
func TestSendWaitingTransactions_StopsBeforeStartingABatchItCannotFinish(t *testing.T) {
	repo := &swSpyKnownTxRepo{pages: [][]*entity.KnownTxForStatusSync{{
		waitingTx("aa", nil), waitingTx("bb", nil), waitingTx("cc", nil),
	}}}
	p := newSweepProcess(t, repo)

	// Enough budget left to read the page, far too little to start a batch.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	result, err := p.SendWaitingTransactions(ctx, 0)

	// Deferring the remainder is not a failure: reporting it as one makes the monitor
	// log "Task failed" for an ordinary backlog.
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Zero(t, repo.statusCalls, "no batch may be started once the budget cannot cover one")
}

// TestSendWaitingTransactions_ContextErrorIsNotAHardFailure covers the batch that does
// start inside the budget and then dies on it. That is the run running out of time, not
// a defect in the batch, so it must not reach the joined error.
func TestSendWaitingTransactions_ContextErrorIsNotAHardFailure(t *testing.T) {
	for name, ctxErr := range map[string]error{
		"deadline exceeded": context.DeadlineExceeded,
		"canceled":          context.Canceled,
	} {
		t.Run(name, func(t *testing.T) {
			repo := &swSpyKnownTxRepo{
				pages:     [][]*entity.KnownTxForStatusSync{{waitingTx("aa", nil), waitingTx("bb", nil)}},
				statusErr: ctxErr,
			}
			p := newSweepProcess(t, repo)

			// No deadline on the context: the budget guard lets every batch start, so the
			// context error can only come from the batch itself.
			_, err := p.SendWaitingTransactions(context.Background(), 0)

			require.NoError(t, err)
			assert.Equal(t, 2, repo.statusCalls, "a context error in one batch must not stop the others")
		})
	}
}

// TestSendWaitingTransactions_RealBatchErrorStillJoins pins the other half of the
// contract: filtering context errors must not swallow genuine ones.
func TestSendWaitingTransactions_RealBatchErrorStillJoins(t *testing.T) {
	boom := errors.New("boom: the database is on fire")
	repo := &swSpyKnownTxRepo{
		pages:     [][]*entity.KnownTxForStatusSync{{waitingTx("aa", nil), waitingTx("bb", nil)}},
		statusErr: boom,
	}
	p := newSweepProcess(t, repo)

	_, err := p.SendWaitingTransactions(context.Background(), 0)

	require.Error(t, err)
	require.ErrorIs(t, err, boom)
	assert.Equal(t, 2, repo.statusCalls, "a failing batch must not strand the rest")
}

func TestCollectWaitingBatches(t *testing.T) {
	batch := "shared-batch"

	t.Run("groups by batch and gives every unbatched tx its own", func(t *testing.T) {
		repo := &swSpyKnownTxRepo{pages: [][]*entity.KnownTxForStatusSync{{
			waitingTx("aa", &batch), waitingTx("bb", &batch), waitingTx("cc", nil),
		}}}
		p := newSweepProcess(t, repo)

		batches, err := p.collectWaitingBatches(context.Background(), p.logger, queryopts.Until{Time: time.Now()})

		require.NoError(t, err)
		require.Len(t, batches, 2)
		assert.Equal(t, batch, batches[0].name)
		assert.Equal(t, []string{"aa", "bb"}, batches[0].txIDs)
		assert.Equal(t, waitingBatch{name: "cc", txIDs: []string{"cc"}}, batches[1], "an unbatched tx is keyed by its own txID")
	})

	// The repo returns rows oldest-first; the sweep has to broadcast them in that order,
	// so a batch takes the position of its oldest member.
	t.Run("keeps the order the rows were read in", func(t *testing.T) {
		late := "late-batch"
		repo := &swSpyKnownTxRepo{pages: [][]*entity.KnownTxForStatusSync{{
			waitingTx("oldest", nil),
			waitingTx("second", &batch),
			waitingTx("third", nil),
			waitingTx("fourth", &batch),
			waitingTx("fifth", &late),
		}}}
		p := newSweepProcess(t, repo)

		batches, err := p.collectWaitingBatches(context.Background(), p.logger, queryopts.Until{Time: time.Now()})

		require.NoError(t, err)
		assert.Equal(t, []waitingBatch{
			{name: "oldest", txIDs: []string{"oldest"}},
			{name: batch, txIDs: []string{"second", "fourth"}},
			{name: "third", txIDs: []string{"third"}},
			{name: late, txIDs: []string{"fifth"}},
		}, batches)
	})

	t.Run("a failing first page is a hard error when nothing was read", func(t *testing.T) {
		repo := &swSpyKnownTxRepo{pageErrs: []error{errors.New("boom: query failed")}}
		p := newSweepProcess(t, repo)

		_, err := p.collectWaitingBatches(context.Background(), p.logger, queryopts.Until{Time: time.Now()})

		require.Error(t, err)
	})

	t.Run("keeps the pages already read when a later page fails", func(t *testing.T) {
		full := make([]*entity.KnownTxForStatusSync, 0, sendWaitingItemsPerPage)
		for i := range sendWaitingItemsPerPage {
			full = append(full, waitingTx(string(rune('a'+i%26))+strconv.Itoa(i), nil))
		}
		repo := &swSpyKnownTxRepo{
			pages:    [][]*entity.KnownTxForStatusSync{full},
			pageErrs: []error{nil, errors.New("boom: page two failed")},
		}
		p := newSweepProcess(t, repo)

		batches, err := p.collectWaitingBatches(context.Background(), p.logger, queryopts.Until{Time: time.Now()})

		// Broadcasting what was already read still makes progress; the rest waits for
		// the next run.
		require.NoError(t, err)
		assert.Len(t, batches, sendWaitingItemsPerPage)
	})

	t.Run("stops paging once the context is done", func(t *testing.T) {
		repo := &swSpyKnownTxRepo{}
		p := newSweepProcess(t, repo)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		batches, err := p.collectWaitingBatches(ctx, p.logger, queryopts.Until{Time: time.Now()})

		require.NoError(t, err)
		assert.Empty(t, batches)
		assert.Zero(t, repo.pageCalls)
	})
}

func TestSweepBudget(t *testing.T) {
	t.Run("allows everything when the context carries no deadline", func(t *testing.T) {
		b := newSweepBudget(context.Background())
		assert.True(t, b.allows(context.Background()))
	})

	t.Run("refuses once the remaining time is below the reserve", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		b := newSweepBudget(ctx)
		assert.False(t, b.allows(ctx), "a reserve of %s cannot fit in 100ms", b.reserve())
	})

	t.Run("allows while the deadline is comfortably far away", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		b := newSweepBudget(ctx)
		assert.True(t, b.allows(ctx))
	})

	t.Run("refuses a done context even without a deadline", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		b := newSweepBudget(context.Background())
		assert.False(t, b.allows(ctx))
	})

	t.Run("reserve never drops below the floor, however fast the batches are", func(t *testing.T) {
		b := newSweepBudget(context.Background())
		for range 50 {
			b.observe(time.Millisecond)
		}
		assert.Equal(t, sweepMinReserve, b.reserve())
	})

	t.Run("reserve grows with slow batches", func(t *testing.T) {
		b := newSweepBudget(context.Background())
		for range 50 {
			b.observe(30 * time.Second)
		}
		assert.Greater(t, b.reserve(), sweepMinReserve)
	})
}

func TestIsContextErr(t *testing.T) {
	assert.True(t, isContextErr(context.DeadlineExceeded))
	assert.True(t, isContextErr(context.Canceled))
	assert.True(t, isContextErr(errors.Join(errors.New("wrapped"), context.DeadlineExceeded)))
	assert.False(t, isContextErr(errors.New("boom: something else")))
	assert.False(t, isContextErr(nil))
}
