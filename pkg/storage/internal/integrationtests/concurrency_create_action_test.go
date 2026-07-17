package integrationtests

import (
	"sync"
	"testing"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testmode"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TestConcurrentCreateActionSameProvidedOutpoint_ExactlyOneSucceeds encodes the
// provided-input double-claim (Decision Record v1, W0-3; discussion #933).
//
// It is INTENTIONALLY RED on current code: user-provided NON-change inputs route
// to KnownOutputIDs and are claimed only by markReservedOutputsAsNotSpendable,
// whose `spent_by IS NULL` UPDATE ignores RowsAffected. The spendability check is
// an in-memory read on the base pool BEFORE the funding txn opens, so N concurrent
// CreateActions all pass the read and all commit — the losers' UPDATE silently
// matches 0 rows. W1-1 (RowsAffected equality) turns this test green.
//
// Postgres-only: the race needs true row-level concurrency, which the SQLite
// single-connection test pin cannot provide.
func TestConcurrentCreateActionSameProvidedOutpoint_ExactlyOneSucceeds(t *testing.T) {
	if _, ok := testmode.GetMode().(*testmode.PostgresMode); !ok {
		t.Skip("requires TEST_DB_MODE=postgres (see docs/testing-postgres.md)")
	}

	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().GORM()
	ctx := t.Context()

	// given: Alice owns ONE known, NON-change, spendable output O, large enough to
	// self-fund each transaction (so there is no change-UTXO contention and the ONLY
	// shared resource under contention is O itself).
	//
	// O is created by internalizing a large P2PKH output into a CUSTOM basket
	// (BasketInsertionProtocol => Change=false => routes through KnownOutputIDs and,
	// crucially, never gets a bsv_user_utxos row).
	const providedOutputSatoshis = 100_000
	seedSpec := txtestabilities.GivenTX().
		WithInput(providedOutputSatoshis + 1_000).
		WithP2PKHOutput(providedOutputSatoshis)
	seedBeef := seedSpec.AtomicBEEF().Bytes()

	internalizeResult, err := activeStorage.InternalizeAction(ctx, testusers.Alice.AuthID(), wdk.InternalizeActionArgs{
		Tx: seedBeef,
		Outputs: []*wdk.InternalizeOutput{
			{
				OutputIndex: 0,
				Protocol:    wdk.BasketInsertionProtocol,
				InsertionRemittance: &wdk.BasketInsertion{
					Basket: "concurrency-double-claim-basket",
				},
			},
		},
		Description: "seed non-change spendable output O",
	})
	require.NoError(t, err)
	require.True(t, internalizeResult.Accepted)

	// let the background broadcaster settle so it does not race the assertions below.
	time.Sleep(200 * time.Millisecond)

	providedOutpoint := wdk.OutPoint{TxID: internalizeResult.TxID, Vout: 0}

	// confirm the route: O must have NO bsv_user_utxos row. If it did, it would be a
	// change coin taking the RowsAffected-protected reserveUTXOs path instead of the
	// vulnerable KnownOutputIDs path — and this test would not exercise the bug.
	var utxoRows int64
	require.NoError(t, activeStorage.Database.DB.Raw(`
		SELECT count(*) FROM bsv_user_utxos uu
		JOIN bsv_outputs o ON o.id = uu.output_id
		JOIN bsv_transactions tx ON tx.id = o.transaction_id
		WHERE tx.tx_id = ? AND o.vout = ?`,
		providedOutpoint.TxID, providedOutpoint.Vout,
	).Scan(&utxoRows).Error)
	require.Zerof(t, utxoRows,
		"provided output has %d user_utxos row(s); it would route through the protected change path, not KnownOutputIDs", utxoRows)

	// invariants hold on the healthy seeded state.
	testabilities.AssertStorageInvariants(t, activeStorage.Database.DB)

	// when: N goroutines fire CreateAction, all providing O as their only input,
	// released together via a start barrier to widen the race window.
	const n = 8
	argsFor := func() wdk.ValidCreateActionArgs {
		args := fixtures.DefaultValidCreateActionArgs()
		args.IsSignAction = true
		args.Options.TrustSelf = to.Ptr(sdk.TrustSelfKnown)
		args.Outputs = []wdk.ValidCreateActionOutput{} // O alone funds the tx: no change contention
		args.InputBEEF = seedBeef
		args.Inputs = []wdk.ValidCreateActionInput{{
			Outpoint:              providedOutpoint,
			UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(108)),
			InputDescription:      "provided known non-change input",
		}}
		return args
	}

	var start sync.WaitGroup
	start.Add(1)
	results := make([]error, n)
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

	// then: at most one CreateAction may claim the outpoint.
	successes := 0
	for _, resErr := range results {
		if resErr == nil {
			successes++
		}
	}
	require.Equalf(t, 1, successes,
		"provided-input double-claim: %d CreateActions claimed the same outpoint (expected exactly 1)", successes)

	// and: cross-table money-safety invariants still hold.
	testabilities.AssertStorageInvariants(t, activeStorage.Database.DB)
}
