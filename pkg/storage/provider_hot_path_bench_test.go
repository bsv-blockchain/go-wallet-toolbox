package storage_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	pkgtestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

const (
	benchTopUpSatoshis  = 10_000
	benchOutputSatoshis = 6_000
)

// BenchmarkProvider_CreateAndProcessAction measures one createAction -> sign -> processAction
// round trip at a time, the per-action hot path of a wallet: funding, the spend, the BEEF
// build and script verification, the claim, the broadcast (to the fixture's mocked
// broadcasters) and its result.
//
// Read allocs/op and B/op. Most of ns/op is spent waiting for the fixture's mocked
// broadcasters, so it says little about the storage work.
//
// Logging runs at WARN, as in production, so disabled debug logging (including the SQL trace
// of every statement) costs what it costs there; the test fixture's DEBUG logger would
// otherwise dominate. It runs on the default test database (in-memory SQLite); under
// TEST_DB_MODE=postgres it runs on the test Postgres instead.
//
//	go test -run '^$' -bench Provider_CreateAndProcessAction -benchmem -benchtime 1000x -count 6 ./pkg/storage/
func BenchmarkProvider_CreateAndProcessAction(b *testing.B) {
	given, cleanup := pkgtestabilities.Given(b)
	defer cleanup()

	quiet := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))

	activeStorage := given.Provider().WithLogger(quiet).GORM()

	faucet := given.Faucet(activeStorage, testusers.Alice)
	for range b.N {
		faucet.TopUp(benchTopUpSatoshis)
	}

	keyDeriver := sdk.NewKeyDeriver(testusers.Alice.PrivateKey(b))
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if err := benchCreateAndProcess(ctx, activeStorage, keyDeriver); err != nil {
			b.Fatal(err)
		}
	}
}

func benchCreateAndProcess(ctx context.Context, provider *storage.Provider, keyDeriver *sdk.KeyDeriver) error {
	args := fixtures.DefaultValidCreateActionArgs()
	// More than half of a top-up: the change can never fund the next action, so every action
	// spends a fresh top-up and the unmined ancestry the BEEF build walks stays one deep.
	args.Outputs[0].Satoshis = benchOutputSatoshis

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
