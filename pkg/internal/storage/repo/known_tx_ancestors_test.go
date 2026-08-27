package repo_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Ancestors carried inside a caller's inputBEEF are recorded as rows so the blob
// does not have to be stored once per descendant. The writes are additive: they
// run alongside every other path that writes known txs and must never take
// anything away from a row that already exists.

func TestRegisterAncestors_InsertsUnprovenAncestorAsUnmined(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	ancestorTx := beefTestTx(201)

	require.NoError(t, repos.RegisterAncestors(t.Context(), "ref-1", []entity.AncestorTx{{
		TxID:  ancestorTx.TxID().String(),
		RawTx: ancestorTx.Bytes(),
	}}))

	var row models.KnownTx
	require.NoError(t, db.DB.First(&row, "tx_id = ?", ancestorTx.TxID().String()).Error)
	assert.NotEmpty(t, row.RawTx, "the ancestry walk needs the raw tx to reach this tx's own parents")

	// "unknown" is in ProvenTxReqProblematicStatuses, which every BEEF build
	// filters out - a row written that way exists but is unreachable.
	assert.Equal(t, wdk.ProvenTxStatusUnmined, row.Status)
	assert.NotContains(t, wdk.ProvenTxReqProblematicStatuses, row.Status,
		"a registered ancestor must not be filtered out of BEEF builds")
}

func TestRegisterAncestors_InsertsProvenAncestorAsCompleted(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	ancestorTx := beefTestTx(202)
	proof := testutils.MockValidMerklePath(t, ancestorTx.TxID().String(), 800000)

	require.NoError(t, repos.RegisterAncestors(t.Context(), "ref-2", []entity.AncestorTx{{
		TxID:       ancestorTx.TxID().String(),
		RawTx:      ancestorTx.Bytes(),
		MerklePath: proof.Bytes(),
	}}))

	var row models.KnownTx
	require.NoError(t, db.DB.First(&row, "tx_id = ?", ancestorTx.TxID().String()).Error)
	assert.Equal(t, wdk.ProvenTxStatusCompleted, row.Status)
	assert.NotEmpty(t, row.MerklePath)
}

// The important guarantee: an ancestor arriving inside a BEEF says nothing about
// a status some other path has already established for it.
func TestRegisterAncestors_NeverDowngradesAnExistingRow(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	existing := beefTestTx(203)
	proof := testutils.MockValidMerklePath(t, existing.TxID().String(), 799000)

	// given: a row that is already completed, with a proof
	require.NoError(t, db.DB.Create(&models.KnownTx{
		TxID:       existing.TxID().String(),
		Status:     wdk.ProvenTxStatusCompleted,
		RawTx:      existing.Bytes(),
		MerklePath: proof.Bytes(),
		Notify:     "{}",
	}).Error)

	// when: the same tx arrives inside a BEEF, unproven
	require.NoError(t, repos.RegisterAncestors(t.Context(), "ref-3", []entity.AncestorTx{{
		TxID:  existing.TxID().String(),
		RawTx: existing.Bytes(),
	}}))

	// then: nothing was taken away
	var row models.KnownTx
	require.NoError(t, db.DB.First(&row, "tx_id = ?", existing.TxID().String()).Error)
	assert.Equal(t, wdk.ProvenTxStatusCompleted, row.Status, "status must not be downgraded")
	assert.NotEmpty(t, row.MerklePath, "an existing proof must not be cleared")
	assert.NotEmpty(t, row.RawTx)
}

func TestRegisterAncestors_FillsGenuinelyEmptyColumns(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	existing := beefTestTx(204)
	proof := testutils.MockValidMerklePath(t, existing.TxID().String(), 798000)

	// given: a row with no raw tx and no proof
	require.NoError(t, db.DB.Create(&models.KnownTx{
		TxID:   existing.TxID().String(),
		Status: wdk.ProvenTxStatusUnmined,
		Notify: "{}",
	}).Error)

	// when: the BEEF supplies both
	require.NoError(t, repos.RegisterAncestors(t.Context(), "ref-4", []entity.AncestorTx{{
		TxID:       existing.TxID().String(),
		RawTx:      existing.Bytes(),
		MerklePath: proof.Bytes(),
	}}))

	var row models.KnownTx
	require.NoError(t, db.DB.First(&row, "tx_id = ?", existing.TxID().String()).Error)
	assert.NotEmpty(t, row.RawTx, "an absent raw tx is a gap worth filling")
	assert.NotEmpty(t, row.MerklePath, "an absent proof is a gap worth filling")
}

// Two callers can submit BEEFs sharing an ancestor at the same instant. Insertion
// is ON CONFLICT DO NOTHING rather than read-then-write precisely so that is not
// a race.
func TestRegisterAncestors_ConcurrentRegistrationOfSameAncestor(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	shared := beefTestTx(205)
	ancestors := []entity.AncestorTx{{
		TxID:  shared.TxID().String(),
		RawTx: shared.Bytes(),
	}}

	const writers = 8
	errs := make([]error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = repos.RegisterAncestors(t.Context(), "ref-concurrent", ancestors)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err, "writer %d", i)
	}

	var count int64
	require.NoError(t, db.DB.Model(&models.KnownTx{}).
		Where("tx_id = ?", shared.TxID().String()).Count(&count).Error)
	assert.Equal(t, int64(1), count, "concurrent writers must not duplicate or fail")
}

func TestRegisterAncestors_SkipsEntriesWithoutARawTx(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())

	// A TxIDOnly entry carries no transaction; a row holding only a txid gives
	// the ancestry walk nothing it can use.
	require.NoError(t, repos.RegisterAncestors(t.Context(), "ref-5", []entity.AncestorTx{
		{TxID: beefTestTx(206).TxID().String()},
		{TxID: "", RawTx: []byte{0x01}},
	}))

	var count int64
	require.NoError(t, db.DB.Model(&models.KnownTx{}).Count(&count).Error)
	assert.Zero(t, count)
}
