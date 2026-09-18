package repo_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
)

// Under TrustSelf the build turns every transaction storage holds into a bare
// txid stub: it needs to know the row exists and nothing else - not its raw tx,
// not its proof, not its stored input beef, and not its parents. The reads that
// feed the build have to respect that, or they ship an entire unproven ancestry
// of blobs across the wire for a result that discards all of it.

// queryRecorder captures the SQL of every read issued through db.
type queryRecorder struct {
	mu    sync.Mutex
	stmts []string
}

func recordQueries(t *testing.T, db *gorm.DB) *queryRecorder {
	t.Helper()
	rec := &queryRecorder{}
	capture := func(tx *gorm.DB) {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		rec.stmts = append(rec.stmts, tx.Statement.SQL.String())
	}
	name := "test:record-" + strings.ReplaceAll(t.Name(), "/", "-")
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(name, capture))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(name) })
	return rec
}

func (r *queryRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stmts = nil
}

func (r *queryRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.stmts...)
}

// unprovenChain stores g0 <- g1 <- ... <- g(n-1), none of them proven, each
// carrying a stored input beef the size busy wallets reach.
func unprovenChain(t *testing.T, db *database.Database, n int) []*transaction.Transaction {
	t.Helper()
	blob := make([]byte, 64*1024)

	chain := make([]*transaction.Transaction, 0, n)
	var parent *transaction.Transaction
	for i := range n {
		var tx *transaction.Transaction
		if parent == nil {
			tx = beefTestTx(uint32(500 + i))
		} else {
			tx = beefTestTx(uint32(500+i), parent)
		}
		storeKnownTx(t, db, tx, blob)
		chain = append(chain, tx)
		parent = tx
	}
	return chain
}

// entryFor returns the BEEF's own record for tx - which says whether it carries
// the raw transaction or is only a txid stub - or nil when it is absent.
func entryFor(beef *transaction.Beef, tx *transaction.Transaction) *transaction.BeefTx {
	return beef.Transactions[*tx.TxID()]
}

func TestGetBEEFForTxIDs_TrustSelf_ReadsExistenceOnly(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	chain := unprovenChain(t, db, 6)
	subject := chain[len(chain)-1]

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	rec := recordQueries(t, db.DB)

	// when:
	beef, err := repos.GetBEEFForTxIDs(t.Context(),
		seq.FromSlice([]string{subject.TxID().String()}),
		entity.WithTrustSelf(sdk.TrustSelfKnown),
	)

	// then: the result is what TrustSelf has always produced - a bare stub.
	require.NoError(t, err)
	stub := entryFor(beef, subject)
	require.NotNil(t, stub, "the subject must be in the BEEF")
	assert.Equal(t, transaction.TxIDOnly, stub.DataFormat, "a storage-known subject is a txid stub under TrustSelf")
	for _, ancestor := range chain[:len(chain)-1] {
		assert.Nil(t, entryFor(beef, ancestor), "a stub has no ancestry below it")
	}

	// and: getting there cost one existence read, with no blob in it.
	stmts := rec.snapshot()
	require.Len(t, stmts, 1, "one read decides existence; walking the ancestry is waste:\n%s", strings.Join(stmts, "\n"))
	for _, stmt := range stmts {
		assert.NotContains(t, stmt, "input_beef", "the stored blob is never used by a stub")
		assert.NotContains(t, stmt, "raw_tx", "the raw tx is never used by a stub")
	}
}

func TestGetBEEFForTxIDs_KnownTxIDs_NeedNoRead(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	chain := unprovenChain(t, db, 3)
	subject := chain[len(chain)-1]

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	rec := recordQueries(t, db.DB)

	beef, err := repos.GetBEEFForTxIDs(t.Context(),
		seq.FromSlice([]string{subject.TxID().String()}),
		entity.WithKnownTxIDs(subject.TxID().String()),
	)

	require.NoError(t, err)
	stub := entryFor(beef, subject)
	require.NotNil(t, stub)
	assert.Equal(t, transaction.TxIDOnly, stub.DataFormat)
	assert.Empty(t, rec.snapshot(), "a txid the caller already knows is stubbed without touching storage")
}

// A subject storage does not hold still has to come from services, and its
// storage-held parents are still stubs.
func TestGetBEEFForTxIDs_TrustSelf_UnknownSubjectStillResolvesThroughServices(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	chain := unprovenChain(t, db, 2)
	parent := chain[len(chain)-1]
	external := beefTestTx(900, parent)

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	rec := recordQueries(t, db.DB)

	beef, err := repos.GetBEEFForTxIDs(t.Context(),
		seq.FromSlice([]string{external.TxID().String()}),
		entity.WithTrustSelf(sdk.TrustSelfKnown),
		entity.WithTxGetterFcn(func(_ context.Context, txID string) ([]byte, *transaction.MerklePath, error) {
			require.Equal(t, external.TxID().String(), txID)
			return external.Bytes(), nil, nil
		}),
	)

	require.NoError(t, err)
	got := entryFor(beef, external)
	require.NotNil(t, got)
	assert.NotEqual(t, transaction.TxIDOnly, got.DataFormat, "a subject fetched from services carries its raw tx")

	parentEntry := entryFor(beef, parent)
	require.NotNil(t, parentEntry, "its storage-held parent must be present")
	assert.Equal(t, transaction.TxIDOnly, parentEntry.DataFormat, "and, under TrustSelf, as a stub")

	// The parent is resolved by the build's own single-row lookup, not the
	// prefetch - that read has to be blob-free too.
	for _, stmt := range rec.snapshot() {
		assert.NotContains(t, stmt, "input_beef", "a stub never needs the stored blob, whichever read finds it")
	}
}

// The recorder must itself be trustworthy: without TrustSelf the build really
// does read raw transactions, so a recorder that saw nothing would be broken.
func TestGetBEEFForTxIDs_FullBuild_StillReadsWhatItNeeds(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	chain := unprovenChain(t, db, 3)
	subject := chain[len(chain)-1]

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	rec := recordQueries(t, db.DB)
	rec.reset()

	_, _ = repos.GetBEEFForTxIDs(t.Context(), seq.FromSlice([]string{subject.TxID().String()}))

	stmts := strings.Join(rec.snapshot(), "\n")
	assert.Contains(t, stmts, "raw_tx", "a full build reads raw transactions")
}
