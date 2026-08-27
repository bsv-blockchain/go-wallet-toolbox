package repo_test

import (
	"strings"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
)

// The stored input beef duplicates ancestry that, for a wallet spending its own
// change, already exists as known tx rows - once per descendant, so the cost grows
// with the depth of the chain. It is dropped when a proof lands, but only where the
// ancestry can be rebuilt from those rows.

func ancestryBeef(t *testing.T, txs ...*transaction.Transaction) []byte {
	t.Helper()
	beef := transaction.NewBeefV2()
	for _, tx := range txs {
		_, err := beef.MergeTransaction(tx)
		require.NoError(t, err)
	}
	bytes, err := beef.Bytes()
	require.NoError(t, err)
	return bytes
}

func markMined(t *testing.T, repos *repo.Repositories, txID string) {
	t.Helper()

	const blockHeight = 800000
	// A real BUMP: the pruned rows stay readable through GetBEEF afterwards, and a
	// placeholder would fail to parse there rather than proving anything.
	merklePath := testutils.MockValidMerklePath(t, txID, blockHeight)

	require.NoError(t, repos.UpdateKnownTxAsMined(t.Context(), &entity.KnownTxAsMined{
		TxID:        txID,
		BlockHeight: blockHeight,
		MerklePath:  merklePath.Bytes(),
		MerkleRoot:  "root",
		BlockHash:   "hash",
	}))
}

func storedInputBeef(t *testing.T, db *database.Database, txID string) []byte {
	t.Helper()
	var row models.KnownTx
	require.NoError(t, db.DB.Model(&models.KnownTx{}).
		Select("input_beef").
		Where("tx_id = ?", txID).
		First(&row).Error)
	return row.InputBeef
}

func TestPruneInputBeef_DropsBlobWhenParentsAreKnownRows(t *testing.T) {
	// given: grandparent <- parent <- subject, all known to storage, with the
	// subject carrying the whole ancestry as a blob.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	grandparent := beefTestTx(101)
	parent := beefTestTx(102, grandparent)
	subject := beefTestTx(103, parent)

	storeKnownTx(t, db, grandparent, nil)
	storeKnownTx(t, db, parent, nil)
	storeKnownTx(t, db, subject, ancestryBeef(t, grandparent, parent))

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	require.NotEmpty(t, storedInputBeef(t, db, subject.TxID().String()), "precondition: the blob is stored")

	// when: the proof lands
	markMined(t, repos, subject.TxID().String())

	// then: the duplicated ancestry is gone...
	assert.Empty(t, storedInputBeef(t, db, subject.TxID().String()),
		"ancestry reachable through known tx rows must not also be kept as a blob")

	// ...and the ancestry is still reachable, now via those rows. MinProofLevel is
	// the demanding case: it deliberately walks PAST the subject's fresh proof, so
	// it asks for exactly the ancestry the blob used to be one copy of.
	beef, err := repos.GetBEEFForTxIDs(
		t.Context(),
		seq.FromSlice([]string{subject.TxID().String()}),
		entity.WithMinProofLevel(1),
	)
	require.NoError(t, err)
	assert.NotNil(t, beef.FindTransaction(parent.TxID().String()),
		"the direct parent must still be reachable after pruning")
}

func TestPruneInputBeef_KeepsBlobHoldingCallerSuppliedParent(t *testing.T) {
	// given: the subject spends a transaction that is NOT a known tx. The blob is
	// the only copy of it that exists.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	external := beefTestTx(111)
	subject := beefTestTx(112, external)

	storeKnownTx(t, db, subject, ancestryBeef(t, external))

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())

	// when: the proof lands
	markMined(t, repos, subject.TxID().String())

	// then: the blob survives - a proof on the subject says nothing about whether
	// its parents can be reached any other way.
	assert.NotEmpty(t, storedInputBeef(t, db, subject.TxID().String()),
		"the only copy of a caller-supplied parent must not be pruned")

	beef, err := repos.GetBEEFForTxIDs(
		t.Context(),
		seq.FromSlice([]string{subject.TxID().String()}),
		entity.WithMinProofLevel(1),
	)
	require.NoError(t, err)
	assert.NotNil(t, beef.FindTransaction(external.TxID().String()),
		"a caller-supplied parent must survive the proof")
}

func TestPruneInputBeef_KeepsBlobWhenOnlySomeParentsAreKnown(t *testing.T) {
	// given: two parents, one known to storage and one supplied by the caller.
	// Pruning would lose the second, so the blob has to stay whole.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	known := beefTestTx(121)
	external := beefTestTx(122)
	subject := beefTestTx(123, known, external)

	storeKnownTx(t, db, known, nil)
	storeKnownTx(t, db, subject, ancestryBeef(t, known, external))

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())

	markMined(t, repos, subject.TxID().String())

	assert.NotEmpty(t, storedInputBeef(t, db, subject.TxID().String()),
		"a partially reconstructible ancestry must be kept in full")
}

func TestPruneInputBeef_KeepsBlobProvingAnAncestorTheRowCannot(t *testing.T) {
	// given: the parent is a known row, but only the blob holds its proof - the
	// row itself has no merkle path. The row does not cover the blob.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	parent := beefTestTx(141)
	subject := beefTestTx(142, parent)

	beef := transaction.NewBeefV2()
	_, err := beef.MergeTransaction(parent)
	require.NoError(t, err)
	parentProof := testutils.MockValidMerklePath(t, parent.TxID().String(), 799999)
	bumpIndex := beef.MergeBump(&parentProof)
	require.GreaterOrEqual(t, bumpIndex, 0)
	beef.Transactions[*parent.TxID()].DataFormat = transaction.RawTxAndBumpIndex
	beef.Transactions[*parent.TxID()].BumpIndex = bumpIndex
	blob, err := beef.Bytes()
	require.NoError(t, err)

	storeKnownTx(t, db, parent, nil) // stored without a merkle path
	storeKnownTx(t, db, subject, blob)

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())

	markMined(t, repos, subject.TxID().String())

	assert.NotEmpty(t, storedInputBeef(t, db, subject.TxID().String()),
		"a proof that exists only inside the blob must not be pruned away")
}

func TestPruneInputBeef_DropsBlobWhenTransactionHasNoParents(t *testing.T) {
	// given: nothing is spent, so there is no ancestry the blob could be the only
	// record of.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	subject := beefTestTx(131)
	storeKnownTx(t, db, subject, ancestryBeef(t, subject))

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())

	markMined(t, repos, subject.TxID().String())

	assert.Empty(t, storedInputBeef(t, db, subject.TxID().String()))
}

// Pruning is an optimisation; it must never be able to fail the proof it rides on.
func TestPruneInputBeef_ProofSucceedsWhenRawTxIsUnusable(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())

	tests := map[string]models.KnownTx{
		"raw tx is absent":     {TxID: strings.Repeat("a1", 32), RawTx: nil, InputBeef: []byte{0xAA}},
		"raw tx is unparsable": {TxID: strings.Repeat("b2", 32), RawTx: []byte{0xDE, 0xAD}, InputBeef: []byte{0xBB}},
	}

	for name, row := range tests {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, db.DB.Create(&row).Error)

			markMined(t, repos, row.TxID)

			// The proof applied, and the blob was left alone rather than guessed at.
			assert.NotEmpty(t, storedInputBeef(t, db, row.TxID))
		})
	}
}
