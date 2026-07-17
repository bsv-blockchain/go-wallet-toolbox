package funder_test

import (
	"context"
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// boundedCall records one bounded-query invocation made by the funder, including a
// snapshot of the exclusion list at call time (the funder mutates its own slice, so
// the fake must copy it).
type boundedCall struct {
	kind     string // "sufficient" | "insufficient"
	status   wdk.UTXOStatus
	bound    uint64
	excluded []uint
}

// boundedFakeUTXORepository is an in-memory funder.UTXORepository implementing the
// two bounded queries with the same ordering semantics as the real repository
// (sufficient: satoshis ASC, output_id ASC, first >= min; insufficient: satoshis
// DESC, output_id DESC, all < max, capped at limit). Crucially it keeps returning
// the same rows on every call unless they appear in the excluded list — mirroring
// the real repository, where allocated rows keep reserved_by_id NULL until
// reserveUTXOs runs and our own locks don't block our own queries. A funder that
// fails to grow its exclusion list would therefore allocate the same row twice.
type boundedFakeUTXORepository struct {
	rows  []*models.UserUTXO
	calls []boundedCall
}

func (f *boundedFakeUTXORepository) FindNotReservedUTXOsForUpdate(
	_ context.Context, _ *gorm.DB, _ int, _ string, _ *queryopts.Paging, _ []uint, _ bool,
) ([]*models.UserUTXO, error) {
	panic("exhaustive pager must not be called on the non-sweep path")
}

func (f *boundedFakeUTXORepository) FindSmallestSufficientUTXOForUpdate(
	_ context.Context, _ *gorm.DB, _ int, _ string,
	status wdk.UTXOStatus, minSatoshis uint64, excludedOutputIDs []uint,
) (*models.UserUTXO, error) {
	f.record("sufficient", status, minSatoshis, excludedOutputIDs)

	var best *models.UserUTXO
	for _, r := range f.eligible(status, excludedOutputIDs) {
		if r.Satoshis < minSatoshis {
			continue
		}
		if best == nil || r.Satoshis < best.Satoshis || (r.Satoshis == best.Satoshis && r.OutputID < best.OutputID) {
			best = r
		}
	}
	return best, nil
}

func (f *boundedFakeUTXORepository) FindLargestInsufficientUTXOsForUpdate(
	_ context.Context, _ *gorm.DB, _ int, _ string,
	status wdk.UTXOStatus, maxSatoshis uint64, limit int, excludedOutputIDs []uint,
) ([]*models.UserUTXO, error) {
	f.record("insufficient", status, maxSatoshis, excludedOutputIDs)

	var result []*models.UserUTXO
	for _, r := range f.eligible(status, excludedOutputIDs) {
		if r.Satoshis < maxSatoshis {
			result = append(result, r)
		}
	}
	sort.Slice(result, func(a, b int) bool {
		if result[a].Satoshis != result[b].Satoshis {
			return result[a].Satoshis > result[b].Satoshis
		}
		return result[a].OutputID > result[b].OutputID
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (f *boundedFakeUTXORepository) eligible(status wdk.UTXOStatus, excluded []uint) []*models.UserUTXO {
	var result []*models.UserUTXO
	for _, r := range f.rows {
		if r.UTXOStatus != status || slices.Contains(excluded, r.OutputID) {
			continue
		}
		result = append(result, r)
	}
	return result
}

func (f *boundedFakeUTXORepository) record(kind string, status wdk.UTXOStatus, bound uint64, excluded []uint) {
	f.calls = append(f.calls, boundedCall{
		kind:     kind,
		status:   status,
		bound:    bound,
		excluded: slices.Clone(excluded),
	})
}

func (f *boundedFakeUTXORepository) queriedStatuses() []wdk.UTXOStatus {
	statuses := make([]wdk.UTXOStatus, 0, len(f.calls))
	for _, c := range f.calls {
		statuses = append(statuses, c.status)
	}
	return statuses
}

func boundedUTXO(outputID uint, sats uint64, status wdk.UTXOStatus) *models.UserUTXO {
	return &models.UserUTXO{
		UserID:             testusers.Alice.ID,
		OutputID:           outputID,
		BasketName:         wdk.BasketNameForChange,
		UTXOStatus:         status,
		Satoshis:           sats,
		EstimatedInputSize: txutils.P2PKHEstimatedInputSize,
	}
}

func newBoundedFunder(t *testing.T, repo funder.UTXORepository, satPerKB int64) *funder.SQL {
	t.Helper()
	feeModel := defs.FeeModel{Type: defs.SatPerKB, Value: satPerKB}
	return funder.NewSQL(logging.NewTestLogger(t), repo, feeModel, defs.DefaultChangeBasket().MaxChangeOutputsPerTx)
}

func boundedTestBasket() *entity.OutputBasket {
	return &entity.OutputBasket{
		UserID:                  testusers.Alice.ID,
		Name:                    wdk.BasketNameForChange,
		NumberOfDesiredUTXOs:    30,
		MinimumDesiredUTXOValue: 1000,
	}
}

func allocatedIDs(result *funder.Result) []uint {
	ids := make([]uint, 0, len(result.AllocatedUTXOs))
	for _, u := range result.AllocatedUTXOs {
		ids = append(ids, u.OutputID)
	}
	return ids
}

// TestFunderBounded_FeeGrowthRecomputesRemaining pins the CRITICAL edge of the
// bounded loop: allocating an input grows the transaction size, so the fee — and
// therefore the remaining need — must be recomputed before EVERY query.
//
// At 1000 sat/kB, target 10000, txSize 44: initial need is 10044. Rows: 8000, 2400,
// 2100 (all mined, all insufficient). Allocating 8000 grows the fee by 148 sats, so
// the true remaining is 10044-8000+148 = 2192 — making 2100 INSUFFICIENT even though
// it would be sufficient against the stale remaining of 10044-8000 = 2044. A loop
// that recomputes correctly must fund with exactly [8000, 2400]; a loop using the
// stale remaining would pick 2100 second, come up short, and need a third input.
func TestFunderBounded_FeeGrowthRecomputesRemaining(t *testing.T) {
	repo := &boundedFakeUTXORepository{rows: []*models.UserUTXO{
		boundedUTXO(1, 8000, wdk.UTXOStatusMined),
		boundedUTXO(2, 2400, wdk.UTXOStatusMined),
		boundedUTXO(3, 2100, wdk.UTXOStatusMined),
	}}
	funderSvc := newBoundedFunder(t, repo, 1000)

	result, err := funderSvc.Fund(t.Context(), 10_000, 44, 1, boundedTestBasket(), testusers.Alice.ID, nil, nil, false, false, 0, nil)

	require.NoError(t, err)
	require.Equal(t, []uint{1, 2}, allocatedIDs(result),
		"fee growth must be reflected in remaining: 2400 (smallest >= 2192) must be chosen, not 2100 (sufficient only against the stale 2044)")
}

// TestFunderBounded_DrainStopsWhenBatchRowBecomesSufficient pins the batch
// consumption rule: rows from the largest-insufficient batch are consumed only
// while they are still strictly below the freshly recomputed remaining. Once the
// next batch row would be sufficient, the funder must stop draining and re-run the
// sufficient query so the SMALLEST sufficient row is picked (per-step equivalence
// with the old in-memory selectBest), not the batch row.
//
// Rows 300, 200, 150, 100 (mined), need 550: drain 300 (remaining 250) then 200
// (remaining 50). The next batch row 150 is now sufficient — old selectBest would
// pick the smallest sufficient row, 100. Consuming the batch greedily until funded
// would wrongly take 150.
func TestFunderBounded_DrainStopsWhenBatchRowBecomesSufficient(t *testing.T) {
	repo := &boundedFakeUTXORepository{rows: []*models.UserUTXO{
		boundedUTXO(1, 300, wdk.UTXOStatusMined),
		boundedUTXO(2, 200, wdk.UTXOStatusMined),
		boundedUTXO(3, 150, wdk.UTXOStatusMined),
		boundedUTXO(4, 100, wdk.UTXOStatusMined),
	}}
	funderSvc := newBoundedFunder(t, repo, 1)

	result, err := funderSvc.Fund(t.Context(), 549, 44, 1, boundedTestBasket(), testusers.Alice.ID, nil, nil, false, false, 0, nil)

	require.NoError(t, err)
	require.Equal(t, []uint{1, 2, 4}, allocatedIDs(result),
		"after draining 300+200 the remaining is 50; the smallest sufficient row (100) must be picked, not the next batch row (150)")
}

// TestFunderBounded_ExclusionListGrowsWithAllocations pins two things at once:
// forbiddenOutputIDs are passed through to every query, and every allocated row is
// appended to the exclusion list. The fake keeps returning row 7 (500 sats) unless
// it is excluded; after allocating it once, the remaining need (201) would make the
// same row "sufficient" again — a funder that failed to exclude it would allocate
// it twice and report success. The correct funder must exhaust the pool instead.
func TestFunderBounded_ExclusionListGrowsWithAllocations(t *testing.T) {
	repo := &boundedFakeUTXORepository{rows: []*models.UserUTXO{
		boundedUTXO(7, 500, wdk.UTXOStatusMined),
	}}
	funderSvc := newBoundedFunder(t, repo, 1)

	forbidden := []uint{42}
	_, err := funderSvc.Fund(t.Context(), 700, 44, 1, boundedTestBasket(), testusers.Alice.ID, forbidden, nil, false, false, 0, nil)

	require.ErrorIs(t, err, wdk.ErrNotEnoughFunds,
		"row 7 must not be allocated twice; once excluded, the pool is exhausted")

	require.NotEmpty(t, repo.calls)
	sawPostAllocationCall := false
	for _, call := range repo.calls {
		require.Contains(t, call.excluded, uint(42), "forbiddenOutputIDs must be passed to every query")
		if slices.Contains(call.excluded, uint(7)) {
			sawPostAllocationCall = true
		}
	}
	require.True(t, sawPostAllocationCall, "queries after the allocation must carry the allocated OutputID in the exclusion list")
}

// TestFunderBounded_TierWalkOrder ensures mined is preferred over unproven even
// when the unproven tier holds an exact match: the very first query must target the
// mined tier, and its (larger) row must win.
func TestFunderBounded_TierWalkOrder(t *testing.T) {
	repo := &boundedFakeUTXORepository{rows: []*models.UserUTXO{
		boundedUTXO(1, 101, wdk.UTXOStatusUnproven), // exact match for the 101-sat need
		boundedUTXO(2, 500, wdk.UTXOStatusMined),
	}}
	funderSvc := newBoundedFunder(t, repo, 1)

	result, err := funderSvc.Fund(t.Context(), 100, 44, 1, boundedTestBasket(), testusers.Alice.ID, nil, nil, false, false, 0, nil)

	require.NoError(t, err)
	require.Equal(t, []uint{2}, allocatedIDs(result), "mined 500 must win over the exact-match unproven 101")
	require.Equal(t, []wdk.UTXOStatus{wdk.UTXOStatusMined}, repo.queriedStatuses(),
		"a sufficient mined row must satisfy the call with a single query; unproven must not be probed")
}

// TestFunderBounded_IncludeSendingGate ensures the sending tier is queried only
// when includeSending is true.
func TestFunderBounded_IncludeSendingGate(t *testing.T) {
	t.Run("includeSending=false never queries the sending tier", func(t *testing.T) {
		repo := &boundedFakeUTXORepository{rows: []*models.UserUTXO{
			boundedUTXO(1, 50, wdk.UTXOStatusMined),
			boundedUTXO(2, 500, wdk.UTXOStatusSending), // would fund the call if it were eligible
		}}
		funderSvc := newBoundedFunder(t, repo, 1)

		_, err := funderSvc.Fund(t.Context(), 100, 44, 1, boundedTestBasket(), testusers.Alice.ID, nil, nil, false, false, 0, nil)

		require.ErrorIs(t, err, wdk.ErrNotEnoughFunds)
		require.NotContains(t, repo.queriedStatuses(), wdk.UTXOStatusSending,
			"the sending tier must never be queried when includeSending is false")
	})

	t.Run("includeSending=true funds from the sending tier after mined and unproven", func(t *testing.T) {
		repo := &boundedFakeUTXORepository{rows: []*models.UserUTXO{
			boundedUTXO(2, 500, wdk.UTXOStatusSending),
		}}
		funderSvc := newBoundedFunder(t, repo, 1)

		result, err := funderSvc.Fund(t.Context(), 100, 44, 1, boundedTestBasket(), testusers.Alice.ID, nil, nil, true, false, 0, nil)

		require.NoError(t, err)
		require.Equal(t, []uint{2}, allocatedIDs(result))

		statuses := repo.queriedStatuses()
		require.Contains(t, statuses, wdk.UTXOStatusSending)
		minedIdx := slices.Index(statuses, wdk.UTXOStatusMined)
		unprovenIdx := slices.Index(statuses, wdk.UTXOStatusUnproven)
		sendingIdx := slices.Index(statuses, wdk.UTXOStatusSending)
		require.True(t, minedIdx < unprovenIdx && unprovenIdx < sendingIdx,
			"tiers must be probed safest-first: mined, unproven, then sending")
	})
}

// TestFunderBounded_ExhaustionReturnsErrNotEnoughFunds: a full pass over all tiers
// without a single eligible row must surface wdk.ErrNotEnoughFunds.
func TestFunderBounded_ExhaustionReturnsErrNotEnoughFunds(t *testing.T) {
	repo := &boundedFakeUTXORepository{}
	funderSvc := newBoundedFunder(t, repo, 1)

	_, err := funderSvc.Fund(t.Context(), 100, 44, 1, boundedTestBasket(), testusers.Alice.ID, nil, nil, false, false, 0, nil)

	require.ErrorIs(t, err, wdk.ErrNotEnoughFunds)
	require.Equal(t, []wdk.UTXOStatus{wdk.UTXOStatusMined, wdk.UTXOStatusMined, wdk.UTXOStatusUnproven, wdk.UTXOStatusUnproven}, repo.queriedStatuses(),
		"both queries of both non-sending tiers must be probed before giving up")
}
