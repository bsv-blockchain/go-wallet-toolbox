package funder_test

import (
	"context"
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

// pagedFakeUTXORepository is a fake funder.UTXORepository that hands out a fixed
// sequence of pages regardless of the requested offset/limit. It lets the test
// force loadUTXOPool's exhaustive-page code path (a full-size page followed by a
// shorter one) and simulate the same row reappearing across pages — the
// scenario OFFSET pagination without a fully-unique ORDER BY tiebreaker can
// produce under concurrent lock churn (SELECT ... FOR UPDATE SKIP LOCKED).
type pagedFakeUTXORepository struct {
	pages [][]*models.UserUTXO
	calls int
}

func (f *pagedFakeUTXORepository) FindNotReservedUTXOsForUpdate(
	_ context.Context,
	_ *gorm.DB,
	_ int,
	_ string,
	_ *queryopts.Paging,
	_ []uint,
	_ bool,
) ([]*models.UserUTXO, error) {
	if f.calls >= len(f.pages) {
		return nil, nil
	}
	page := f.pages[f.calls]
	f.calls++
	return page, nil
}

// FindSmallestSufficientUTXOForUpdate satisfies funder.UTXORepository; the sweep-mode
// test never exercises the bounded selection path, so it always reports no match.
func (f *pagedFakeUTXORepository) FindSmallestSufficientUTXOForUpdate(
	_ context.Context,
	_ *gorm.DB,
	_ int,
	_ string,
	_ wdk.UTXOStatus,
	_ uint64,
	_ []uint,
) (*models.UserUTXO, error) {
	return nil, nil
}

// FindLargestInsufficientUTXOsForUpdate satisfies funder.UTXORepository; the sweep-mode
// test never exercises the bounded selection path, so it always reports no rows.
func (f *pagedFakeUTXORepository) FindLargestInsufficientUTXOsForUpdate(
	_ context.Context,
	_ *gorm.DB,
	_ int,
	_ string,
	_ wdk.UTXOStatus,
	_ uint64,
	_ int,
	_ []uint,
) ([]*models.UserUTXO, error) {
	return nil, nil
}

// TestFunderSQLFund_DedupsUTXOsAcrossOverlappingPages drives loadUTXOPool
// indirectly through Fund (loadUTXOPool itself is unexported). It seeds a fake
// repository with a full utxoBatchSize first page — forcing the pager to fetch
// a second page — whose last row ("B") is duplicated as the first row of the
// second page (page2 = [B, C]). This mirrors a row shifting across the OFFSET
// boundary under concurrent lock churn. Without the dedup fix, sweep mode would
// allocate B twice.
func TestFunderSQLFund_DedupsUTXOsAcrossOverlappingPages(t *testing.T) {
	const utxoBatchSize = 1000
	ctx := t.Context()

	page1 := make([]*models.UserUTXO, 0, utxoBatchSize)
	for i := 0; i < utxoBatchSize; i++ {
		page1 = append(page1, &models.UserUTXO{
			UserID:             testusers.Alice.ID,
			OutputID:           uint(i + 1),
			BasketName:         wdk.BasketNameForChange,
			UTXOStatus:         wdk.UTXOStatusMined,
			Satoshis:           uint64(1000 + i),
			EstimatedInputSize: txutils.P2PKHEstimatedInputSize,
		})
	}
	utxoB := page1[len(page1)-1]

	utxoC := &models.UserUTXO{
		UserID:             testusers.Alice.ID,
		OutputID:           utxoBatchSize + 1,
		BasketName:         wdk.BasketNameForChange,
		UTXOStatus:         wdk.UTXOStatusMined,
		Satoshis:           500,
		EstimatedInputSize: txutils.P2PKHEstimatedInputSize,
	}

	page2 := []*models.UserUTXO{utxoB, utxoC}

	repo := &pagedFakeUTXORepository{pages: [][]*models.UserUTXO{page1, page2}}

	feeModel := defs.FeeModel{Type: defs.SatPerKB, Value: 1}
	funderSvc := funder.NewSQL(logging.NewTestLogger(t), repo, feeModel, defs.DefaultChangeBasket().MaxChangeOutputsPerTx)

	basket := &entity.OutputBasket{
		UserID:                  testusers.Alice.ID,
		Name:                    wdk.BasketNameForChange,
		NumberOfDesiredUTXOs:    30,
		MinimumDesiredUTXOValue: 1000,
	}

	// Sweep mode allocates every UTXO left in the pool, so any duplicate
	// returned by loadUTXOPool surfaces directly as a duplicate allocation.
	// tx is nil: the fake repository never dereferences it.
	result, err := funderSvc.Fund(ctx, 0, 44, 1, basket, testusers.Alice.ID, nil, nil, false, true, 0, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	expectedCount := utxoBatchSize + 1 // page1's utxoBatchSize unique rows, plus C (B is a duplicate of the last row of page1)
	require.Len(t, result.AllocatedUTXOs, expectedCount, "duplicated UTXO across pages must be allocated exactly once")

	seen := make(map[uint]struct{}, len(result.AllocatedUTXOs))
	for _, allocated := range result.AllocatedUTXOs {
		_, dup := seen[allocated.OutputID]
		require.Falsef(t, dup, "UTXO with OutputID %d was allocated more than once", allocated.OutputID)
		seen[allocated.OutputID] = struct{}{}
	}
	require.Len(t, seen, expectedCount)
}
