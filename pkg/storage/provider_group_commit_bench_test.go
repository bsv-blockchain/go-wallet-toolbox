package storage_test

import (
	"context"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	glogger "gorm.io/gorm/logger"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	pkgtestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// benchConcurrentCallers is the number of goroutines calling createAction + processAction at once.
const benchConcurrentCallers = 16

// BenchmarkGroupCommit_ConcurrentActions measures concurrent createAction -> sign -> processAction
// round trips (funding, spend, claim and broadcast result: the four grouped phases) with group
// commit off and on. It runs on a file SQLite database in WAL mode with _txlock=immediate and a
// pool of 8 connections, the setup group commit is meant for (the default in-memory test database
// has a single connection and no commit cost to save). Under TEST_DB_MODE=postgres it runs on the
// test Postgres instead.
//
//	go test -run '^$' -bench GroupCommit_ConcurrentActions -benchtime 2000x -count 6 ./pkg/storage/
func BenchmarkGroupCommit_ConcurrentActions(b *testing.B) {
	for _, mode := range []struct {
		name string
		cfg  defs.GroupCommit
	}{
		{name: "off", cfg: defs.GroupCommit{}},
		{name: "on", cfg: defs.GroupCommit{Enabled: true}},
	} {
		b.Run("group_commit="+mode.name, func(b *testing.B) {
			benchConcurrentActions(b, mode.cfg)
		})
	}
}

func benchConcurrentActions(b *testing.B, groupCommit defs.GroupCommit) {
	given, cleanup := pkgtestabilities.Given(b, func(cfg *defs.Database) {
		if cfg.Engine != defs.DBTypeSQLite {
			return
		}
		cfg.SQLite.ConnectionString = "file:" + filepath.Join(b.TempDir(), "bench.sqlite") + "?_txlock=immediate"
		cfg.MaxOpenConnections = 8
		cfg.MaxIdleConnections = 8
	})
	defer cleanup()

	// Quiet logging: the test logger formats every statement at DEBUG, which would dominate.
	activeStorage := given.Provider().
		WithLogger(slog.New(slog.DiscardHandler)).
		WithGroupCommit(groupCommit).
		GORM()
	activeStorage.Database.DB.Logger = glogger.Discard

	faucet := given.Faucet(activeStorage, testusers.Alice)
	for range b.N {
		faucet.TopUp(10_000)
	}

	keyDeriver := sdk.NewKeyDeriver(testusers.Alice.PrivateKey(b))
	ctx := context.Background()
	var failed atomic.Int64
	var firstErr atomic.Value

	batchesBefore, unitsBefore := storage.GroupCommitStats(activeStorage)

	b.SetParallelism((benchConcurrentCallers + runtime.GOMAXPROCS(0) - 1) / runtime.GOMAXPROCS(0))
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := benchOneAction(ctx, activeStorage, keyDeriver); err != nil {
				failed.Add(1)
				firstErr.CompareAndSwap(nil, err)
			}
		}
	})

	b.StopTimer()
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "actions/s")
	if batches, units := storage.GroupCommitStats(activeStorage); batches > batchesBefore {
		b.ReportMetric(float64(units-unitsBefore)/float64(batches-batchesBefore), "units/batch")
	}
	if n := failed.Load(); n > 0 {
		b.Fatalf("%d of %d actions failed, first: %v", n, b.N, firstErr.Load())
	}
}

func benchOneAction(ctx context.Context, provider wdk.WalletStorageProvider, keyDeriver *sdk.KeyDeriver) error {
	args := fixtures.DefaultValidCreateActionArgs()
	args.Outputs[0].Satoshis = 1

	created, err := provider.CreateAction(ctx, testusers.Alice.AuthID(), args)
	if err != nil {
		return err
	}

	signed, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, nil, created).Assemble()
	if err != nil {
		return err
	}
	if err = signed.Sign(); err != nil {
		return err
	}

	_, err = provider.ProcessAction(ctx, testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:   true,
		Reference: to.Ptr(created.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(signed.TxID().String())),
		RawTx:     signed.Bytes(),
	})
	return err
}
