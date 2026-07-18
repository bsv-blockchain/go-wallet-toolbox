package funder_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// BenchmarkSQLFund tracks funder.SQL.Fund under the post–Track-P bounded,
// target-aware selection (per-allocation micro-queries locking only the rows
// considered). The pre–Task-7 whole-pool-loading numbers remain recorded as the
// historical baseline in the SDD ledger for comparison. Sub-benchmarks pool_1000
// and pool_10000 seed a fixed pool of UserUTXO rows for a single user/basket and
// repeatedly call Fund with a target that forces multi-UTXO accumulation; cost
// is now flat in pool size.
//
// Each iteration runs Fund inside its own DB transaction which is rolled back
// (never committed) so the seeded pool stays constant across iterations.
func BenchmarkSQLFund(b *testing.B) {
	b.Run("pool_1000", func(b *testing.B) {
		benchmarkSQLFund(b, 1000)
	})
	b.Run("pool_10000", func(b *testing.B) {
		benchmarkSQLFund(b, 10000)
	})
}

func benchmarkSQLFund(b *testing.B, poolSize int) {
	const (
		target      = satoshi.Value(75_000)
		txSize      = uint64(44)
		outputCount = uint64(1)
		minSats     = int64(500)
		maxSats     = int64(50_000)
	)

	ctx := b.Context()

	given, _, cleanup := testabilities.New(b)
	defer cleanup()

	funderSvc := given.NewFunderService()

	basket := &entity.OutputBasket{
		Name:                    wdk.BasketNameForChange,
		UserID:                  testusers.Alice.ID,
		NumberOfDesiredUTXOs:    32,
		MinimumDesiredUTXOValue: 1000,
	}

	// Seed: N UserUTXO rows for one user/basket, satoshis spread minSats..maxSats,
	// status mined, unreserved.
	spread := maxSats - minSats
	for i := 0; i < poolSize; i++ {
		sats := minSats + (int64(i)*spread)/int64(poolSize)
		given.UTXO().
			InBasket(basket).
			OwnedBy(testusers.Alice).
			WithSatoshis(sats).
			P2PKH().
			WithStatus(wdk.UTXOStatusMined).
			Stored()
	}

	db := given.GormDB()

	fund := func() error {
		tx := db.Begin()
		defer tx.Rollback()

		result, err := funderSvc.Fund(ctx, target, txSize, outputCount, basket, testusers.Alice.ID, nil, nil, false, false, 0, tx)
		if err != nil {
			return err
		}
		if len(result.AllocatedUTXOs) == 0 {
			b.Fatal("BenchmarkSQLFund: Fund succeeded without allocating any UTXOs, benchmark would be measuring a no-op path")
		}
		return nil
	}

	// Pre-flight: verify Fund actually succeeds (and allocates UTXOs) against the
	// seeded pool before timing. A benchmark of an erroring path is worthless.
	if err := fund(); err != nil {
		b.Fatalf("BenchmarkSQLFund: pre-flight Fund failed: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := fund(); err != nil {
			b.Fatalf("BenchmarkSQLFund: Fund failed on iteration %d: %v", i, err)
		}
	}
}
