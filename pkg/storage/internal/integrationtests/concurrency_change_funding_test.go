package integrationtests

import (
	"sync"
	"testing"
	"time"

	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testmode"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// changeUTXOCount (M) is the number of change UTXOs seeded for Alice, and
// concurrentActions (N) is the number of CreateActions racing for them (N > M).
// changeUTXOSatoshis (~20_000) comfortably covers fundedOutputSatoshis (15_000)
// plus fee with a wide margin (fee is ~1 sat at the fixture's 1 sat/KB model), so
// each successful CreateAction always consumes exactly ONE change UTXO — never two.
const (
	changeUTXOCount      = 4
	concurrentActions    = 8
	changeUTXOSatoshis   = 20_000
	fundedOutputSatoshis = 15_000
)

// TestConcurrentChangeFunding_ExactlyMSucceed encodes the change-path UTXO-pool
// contention scenario (Decision Record v1 Track P; T7 bounded funder + T8 bounded
// retry-on-contention): M change UTXOs sit in the shared pool, N > M concurrent
// CreateActions each need exactly one of them (automatic funder selection, no
// explicit inputs provided), so exactly M must succeed and every other attempt
// must fail cleanly with wdk.ErrNotEnoughFunds — the pool must never be
// double-allocated to more than M winners.
//
// Postgres-only: the race needs true row-level concurrency (SELECT ... FOR UPDATE
// SKIP LOCKED), which the SQLite single-connection test pin cannot provide.
func TestConcurrentChangeFunding_ExactlyMSucceed(t *testing.T) {
	if _, ok := testmode.GetMode().(*testmode.PostgresMode); !ok {
		t.Skip("requires TEST_DB_MODE=postgres (see docs/testing-postgres.md)")
	}

	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().GORM()
	ctx := t.Context()

	// given: Alice owns M change UTXOs (each changeUTXOSatoshis), internalized in a
	// single InternalizeAction call as WalletPaymentProtocol outputs (change basket)
	// of one seed transaction — see makeOutputs' WalletPaymentProtocol case, which
	// routes these to BasketNameForChange with Change=true, giving each a
	// bsv_user_utxos row the funder's automatic pool selection can see.
	seedSpec := txtestabilities.GivenTX().WithInput(changeUTXOCount*changeUTXOSatoshis + 1_000)
	for range changeUTXOCount {
		seedSpec = seedSpec.WithP2PKHOutput(changeUTXOSatoshis)
	}
	seedBeef := seedSpec.AtomicBEEF().Bytes()

	outputs := make([]*wdk.InternalizeOutput, changeUTXOCount)
	for i := range outputs {
		outputs[i] = &wdk.InternalizeOutput{
			OutputIndex: uint32(i),
			Protocol:    wdk.WalletPaymentProtocol,
			PaymentRemittance: &wdk.WalletPayment{
				DerivationPrefix:  fixtures.DerivationPrefix,
				DerivationSuffix:  fixtures.DerivationSuffix,
				SenderIdentityKey: fixtures.UserIdentityKeyHex,
			},
		}
	}

	internalizeResult, err := activeStorage.InternalizeAction(ctx, testusers.Alice.AuthID(), wdk.InternalizeActionArgs{
		Tx:          seedBeef,
		Outputs:     outputs,
		Description: "seed change utxos for concurrency test",
	})
	require.NoError(t, err)
	require.True(t, internalizeResult.Accepted)

	// and: broadcast the seed tx synchronously (not via the async background
	// broadcaster) so its change UTXOs move from Sending to Unproven before the
	// concurrent phase starts. The funder's automatic pool selection only considers
	// the Mined/Unproven tiers by default (see funder.allocateBounded's `tiers`),
	// so this must have settled deterministically, not raced.
	given.Provider().ARC().WhenQueryingTx(internalizeResult.TxID).WillReturnTransactionWithoutMerklePath()
	_, err = activeStorage.SendWaitingTransactions(ctx, -time.Minute)
	require.NoError(t, err)

	// sanity: exactly M unreserved, Unproven change UTXOs are visible to the funder.
	var seededUnreserved int64
	require.NoError(t, activeStorage.Database.DB.
		Table("bsv_user_utxos").
		Where("reserved_by_id IS NULL AND utxo_status = ?", string(wdk.UTXOStatusUnproven)).
		Count(&seededUnreserved).Error)
	require.Equalf(t, int64(changeUTXOCount), seededUnreserved,
		"expected %d unreserved, unproven change UTXOs after seeding, got %d", changeUTXOCount, seededUnreserved)

	// invariants hold on the healthy seeded state.
	testabilities.AssertStorageInvariants(t, activeStorage.Database.DB)

	// when: N goroutines fire CreateAction with a single self-funded ~15_000-sat
	// P2PKH output and NO explicit inputs, so the funder must automatically select
	// one change UTXO per action, released together via a start barrier to widen
	// the race window over the shared pool.
	argsFor := func() wdk.ValidCreateActionArgs {
		return fixtures.DefaultValidCreateActionArgs(func(args *wdk.ValidCreateActionArgs) {
			args.Outputs[0].Satoshis = fundedOutputSatoshis
			args.Description = "self-funded from change pool"
		})
	}

	var start sync.WaitGroup
	start.Add(1)
	results := make([]error, concurrentActions)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start.Wait()
			_, createErr := activeStorage.CreateAction(ctx, testusers.Alice.AuthID(), argsFor())
			results[i] = createErr
		}(i)
	}
	start.Done()
	wg.Wait()

	// then: exactly M CreateActions succeed (one change UTXO each); every failure is
	// a clean wdk.ErrNotEnoughFunds (pool exhausted after any contention retries,
	// per T8) — never a silent double-allocation of the same coin.
	successes := 0
	for _, resErr := range results {
		if resErr == nil {
			successes++
			continue
		}
		require.ErrorIsf(t, resErr, wdk.ErrNotEnoughFunds,
			"expected losing CreateAction to fail with wdk.ErrNotEnoughFunds, got: %v", resErr)
	}
	require.Equalf(t, changeUTXOCount, successes,
		"expected exactly M=%d CreateActions to succeed (N=%d racing for M UTXOs), got %d successes",
		changeUTXOCount, concurrentActions, successes)

	// and: exactly M UTXOs got reserved, each by a distinct transaction — the
	// allocated output sets of the M winners never overlap.
	var reservedByIDs []uint
	require.NoError(t, activeStorage.Database.DB.
		Table("bsv_user_utxos").
		Where("reserved_by_id IS NOT NULL").
		Pluck("reserved_by_id", &reservedByIDs).Error)
	require.Lenf(t, reservedByIDs, changeUTXOCount, "expected exactly %d reserved user_utxos rows, got %d", changeUTXOCount, len(reservedByIDs))

	distinctReservers := make(map[uint]struct{}, len(reservedByIDs))
	for _, id := range reservedByIDs {
		distinctReservers[id] = struct{}{}
	}
	require.Lenf(t, distinctReservers, changeUTXOCount,
		"expected %d distinct reserved_by_id values (no shared allocation), got %d", changeUTXOCount, len(distinctReservers))

	// and: cross-table money-safety invariants still hold.
	testabilities.AssertStorageInvariants(t, activeStorage.Database.DB)
}
