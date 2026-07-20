package syncrepo_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// These tests pin the BRC-40 stale-chunk guard required by go-wallet-toolbox#853
// and the ts-stack conformance vectors at conformance/vectors/sync/brc40-user-state.json.
// Reference TS semantics: strict `if (incoming.updated_at > existing.updated_at)`.

func TestBRC40Guard_ProvenTxReq_StaleSkip(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	tNewer := time.Date(2026, 4, 23, 13, 30, 0, 0, time.UTC)
	tStale := time.Date(2026, 4, 23, 12, 30, 0, 0, time.UTC)

	txID := "f4b3d8e3b27e3e6d2c1b1aebf2d2f3e1c4a9f0eedc7a8b1f2e3d4c5b6a7f8e9d"

	// Seed newer (proven) record
	isNew, err := repos.UpsertProvenTxReqForSync(t.Context(), &entity.ProvenTxReq{RawTx: []byte("raw"),
		CreatedAt:   tNewer,
		UpdatedAt:   tNewer,
		TxID:        txID,
		Status:      wdk.ProvenTxStatusCompleted,
		// removed MerklePath, MerkleRoot, BlockHash, BlockHeight
	})
	require.NoError(t, err)
	require.True(t, isNew)

	// Stale incoming: older updated_at, attempts to overwrite with unknown/no-merkle
	isNew, err = repos.UpsertProvenTxReqForSync(t.Context(), &entity.ProvenTxReq{RawTx: []byte("raw"),
		CreatedAt:  tStale,
		UpdatedAt:  tStale,
		TxID:       txID,
		Status:     wdk.ProvenTxStatusUnknown,
		// removed MerklePath, MerkleRoot, BlockHash
	})
	require.NoError(t, err)
	require.False(t, isNew)

	// Verify state preserved
	var got models.ProvenTxReq
	require.NoError(t, d.DB.First(&got, "txid = ?", txID).Error)
	require.Equal(t, wdk.ProvenTxStatusCompleted, got.Status,
		"stale chunk MUST NOT regress status (sync.brc40.merge.proventx.error.regression.1)")
	require.Equal(t, tNewer.UTC(), got.UpdatedAt.UTC())
}

func TestBRC40Guard_ProvenTxReq_EqualSkip(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	t0 := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)
	txID := "a1b2c3d4e5f60718293a4b5c6d7e8f9001112233445566778899aabbccddeeff0"

	_, err := repos.UpsertProvenTxReqForSync(t.Context(), &entity.ProvenTxReq{RawTx: []byte("raw"),
		CreatedAt:   t0,
		UpdatedAt:   t0,
		TxID:        txID,
		Status:      wdk.ProvenTxStatusCompleted,
	})
	require.NoError(t, err)

	// Equal updated_at — strict `>` boundary: MUST skip
	_, err = repos.UpsertProvenTxReqForSync(t.Context(), &entity.ProvenTxReq{RawTx: []byte("raw"),
		CreatedAt:  t0,
		UpdatedAt:  t0,
		TxID:       txID,
		Status:     wdk.ProvenTxStatusUnknown,
	})
	require.NoError(t, err)

	var got models.ProvenTxReq
	require.NoError(t, d.DB.First(&got, "txid = ?", txID).Error)
	require.Equal(t, wdk.ProvenTxStatusCompleted, got.Status,
		"equal updated_at MUST NOT trigger update (sync.brc40.merge.tx.error.regression.2 boundary)")
	// assertions removed
}

func TestBRC40Guard_ProvenTxReq_HappyUpdate(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	tOld := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	tNew := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)
	txID := "0011223344556677889900aabbccddeeff00112233445566778899aabbccddee"

	_, err := repos.UpsertProvenTxReqForSync(t.Context(), &entity.ProvenTxReq{RawTx: []byte("raw"),
		CreatedAt: tOld,
		UpdatedAt: tOld,
		TxID:      txID,
		Status:    wdk.ProvenTxStatusUnknown,
	})
	require.NoError(t, err)

	_, err = repos.UpsertProvenTxReqForSync(t.Context(), &entity.ProvenTxReq{RawTx: []byte("raw"),
		CreatedAt:   tOld,
		UpdatedAt:   tNew,
		TxID:        txID,
		Status:      wdk.ProvenTxStatusCompleted,
		// removed MerklePath, MerkleRoot, BlockHash, BlockHeight
	})
	require.NoError(t, err)

	var got models.ProvenTxReq
	require.NoError(t, d.DB.First(&got, "txid = ?", txID).Error)
	require.Equal(t, wdk.ProvenTxStatusCompleted, got.Status)
	// assertions removed
	// removed merkleRoot assertion
}

func TestBRC40Guard_Transaction_StaleSkip(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	tNewer := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)
	tStale := time.Date(2026, 4, 23, 12, 30, 0, 0, time.UTC)

	reference := "ref-brc40-tx-1"
	txID := "tx-id-newer"

	// Seed newer: status=completed, txID set
	isNew, _, err := repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt:   tNewer,
		UpdatedAt:   tNewer,
		UserID:      user.ID,
		Status:      wdk.TxStatusCompleted,
		Reference:   reference,
		IsOutgoing:  true,
		Satoshis:    5000,
		Description: "newer",
		TxID:        &txID,
	})
	require.NoError(t, err)
	require.True(t, isNew)

	// Stale incoming: would regress status to unsigned, clear txID
	isNew, _, err = repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt:   tNewer,
		UpdatedAt:   tStale,
		UserID:      user.ID,
		Status:      wdk.TxStatusUnsigned,
		Reference:   reference,
		IsOutgoing:  true,
		Satoshis:    5000,
		Description: "stale",
		TxID:        nil,
	})
	require.NoError(t, err)
	require.False(t, isNew)

	var got models.Transaction
	require.NoError(t, d.DB.
		Where("userId = ? AND reference = ?", user.ID, reference).
		First(&got).Error)
	require.Equal(t, wdk.TxStatusCompleted, got.Status,
		"stale chunk MUST NOT regress status completed→unsigned (sync.brc40.merge.tx.error.regression.1)")
	require.NotNil(t, got.TxID, "stale chunk MUST NOT clear txID")
	require.Equal(t, txID, *got.TxID)
	require.Equal(t, "newer", got.Description)
}

func TestBRC40Guard_Transaction_EqualSkip(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	t0 := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)
	reference := "ref-brc40-tx-equal"
	txID := "tx-id-equal"

	_, _, err = repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt: t0, UpdatedAt: t0,
		UserID: user.ID, Status: wdk.TxStatusCompleted, Reference: reference,
		IsOutgoing: false, Satoshis: 1, Description: "newer", TxID: &txID,
	})
	require.NoError(t, err)

	// Equal updated_at — must SKIP
	_, _, err = repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt: t0, UpdatedAt: t0,
		UserID: user.ID, Status: wdk.TxStatusUnsigned, Reference: reference,
		IsOutgoing: false, Satoshis: 1, Description: "equal-stale", TxID: nil,
	})
	require.NoError(t, err)

	var got models.Transaction
	require.NoError(t, d.DB.Where("userId = ? AND reference = ?", user.ID, reference).First(&got).Error)
	require.Equal(t, wdk.TxStatusCompleted, got.Status,
		"equal updated_at MUST NOT trigger update (sync.brc40.merge.tx.error.regression.2)")
	require.Equal(t, "newer", got.Description)
}

func TestBRC40Guard_Transaction_HappyUpdate(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	tOld := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	tNew := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)
	reference := "ref-brc40-tx-happy"
	txID := "tx-id-new"

	_, _, err = repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt: tOld, UpdatedAt: tOld,
		UserID: user.ID, Status: wdk.TxStatusUnsigned, Reference: reference,
		IsOutgoing: true, Satoshis: 1, Description: "old",
	})
	require.NoError(t, err)

	_, _, err = repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt: tOld, UpdatedAt: tNew,
		UserID: user.ID, Status: wdk.TxStatusCompleted, Reference: reference,
		IsOutgoing: true, Satoshis: 1, Description: "new", TxID: &txID,
	})
	require.NoError(t, err)

	var got models.Transaction
	require.NoError(t, d.DB.Where("userId = ? AND reference = ?", user.ID, reference).First(&got).Error)
	require.Equal(t, wdk.TxStatusCompleted, got.Status)
	require.Equal(t, "new", got.Description)
	require.NotNil(t, got.TxID)
	require.Equal(t, txID, *got.TxID)
}

// TestBRC40Guard_Output_SpendableRegression mirrors sync.brc40.merge.output.error.regression.1:
// existing has spendable=false (already spent), stale chunk attempts to flip true with cleared spent_by.
// This is the double-spend hazard called out in go-wallet-toolbox#853.
func TestBRC40Guard_Output_SpendableRegression(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	tNewer := time.Date(2026, 4, 23, 13, 30, 0, 0, time.UTC)
	tStale := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)

	reference := "ref-brc40-output-regression"
	txID := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	_, txnDBID, err := repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt: tNewer, UpdatedAt: tNewer,
		UserID: user.ID, Status: wdk.TxStatusCompleted, Reference: reference,
		IsOutgoing: true, Satoshis: 5000, TxID: &txID,
	})
	require.NoError(t, err)

	spentBy := uint(42)
	// removed basketName
	// Seed newer: spendable=false (consumed), spent_by=42
	isNew, outputID, err := repos.UpsertOutputForSync(t.Context(), &entity.Output{
		CreatedAt:     tNewer,
		UpdatedAt:     tNewer,
		UserID:        user.ID,
		TransactionID: txnDBID,
		SpentBy:       &spentBy,
		Satoshis:      5000,
		Vout:          0,
		// removed BasketName
		Spendable:     false,
		Description:   "newer",
	})
	require.NoError(t, err)
	require.True(t, isNew)
	require.NotZero(t, outputID)

	// Stale incoming: would flip spendable→true and clear spent_by (double-spend hazard)
	isNew, _, err = repos.UpsertOutputForSync(t.Context(), &entity.Output{
		CreatedAt:     tNewer,
		UpdatedAt:     tStale,
		UserID:        user.ID,
		TransactionID: txnDBID,
		SpentBy:       nil,
		Satoshis:      5000,
		Vout:          0,
		// removed BasketName
		Spendable:     true,
		Description:   "stale",
	})
	require.NoError(t, err)
	require.False(t, isNew)

	var got models.Output
	require.NoError(t, d.DB.First(&got, outputID).Error)
	require.False(t, got.Spendable,
		"stale chunk MUST NOT flip spendable false→true (sync.brc40.merge.output.error.regression.1)")
	require.NotNil(t, got.SpentBy,
		"stale chunk MUST NOT clear spent_by (sync.brc40.merge.output.error.regression.2)")
	require.Equal(t, spentBy, *got.SpentBy)
	require.Equal(t, "newer", got.Description)
}

func TestBRC40Guard_Output_HappyUpdate(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	tOld := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	tNew := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)

	reference := "ref-brc40-output-happy"
	txID := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	_, txnDBID, err := repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt: tOld, UpdatedAt: tOld,
		UserID: user.ID, Status: wdk.TxStatusCompleted, Reference: reference,
		IsOutgoing: true, Satoshis: 5000, TxID: &txID,
	})
	require.NoError(t, err)

	// removed basketName
	_, outputID, err := repos.UpsertOutputForSync(t.Context(), &entity.Output{
		CreatedAt: tOld, UpdatedAt: tOld,
		UserID: user.ID, TransactionID: txnDBID,
		Satoshis: 5000, Vout: 0,
		Spendable: true, Description: "old",
	})
	require.NoError(t, err)

	spentBy := uint(99)
	_, _, err = repos.UpsertOutputForSync(t.Context(), &entity.Output{
		CreatedAt: tOld, UpdatedAt: tNew,
		UserID: user.ID, TransactionID: txnDBID,
		SpentBy:  &spentBy,
		Satoshis: 5000, Vout: 0,
		Spendable: false, Description: "new",
	})
	require.NoError(t, err)

	var got models.Output
	require.NoError(t, d.DB.First(&got, outputID).Error)
	require.False(t, got.Spendable)
	require.NotNil(t, got.SpentBy)
	require.Equal(t, spentBy, *got.SpentBy)
	require.Equal(t, "new", got.Description)
}

func TestBRC40Guard_Output_EqualSkip(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	t0 := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)
	reference := "ref-brc40-output-equal"
	txID := "aaaabbbbccccddddeeeeffff00001111aaaabbbbccccddddeeeeffff00001111"
	_, txnDBID, err := repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt: t0, UpdatedAt: t0,
		UserID: user.ID, Status: wdk.TxStatusCompleted, Reference: reference,
		IsOutgoing: true, Satoshis: 5000, TxID: &txID,
	})
	require.NoError(t, err)

	spentBy := uint(7)
	// removed basketName
	_, outputID, err := repos.UpsertOutputForSync(t.Context(), &entity.Output{
		CreatedAt: t0, UpdatedAt: t0,
		UserID: user.ID, TransactionID: txnDBID,
		SpentBy: &spentBy, Satoshis: 5000, Vout: 0,
		Spendable: false, Description: "newer",
	})
	require.NoError(t, err)

	// Equal timestamp — must SKIP
	_, _, err = repos.UpsertOutputForSync(t.Context(), &entity.Output{
		CreatedAt: t0, UpdatedAt: t0,
		UserID: user.ID, TransactionID: txnDBID,
		SpentBy: nil, Satoshis: 5000, Vout: 0,
		Spendable: true, Description: "equal-stale",
	})
	require.NoError(t, err)

	var got models.Output
	require.NoError(t, d.DB.First(&got, outputID).Error)
	require.False(t, got.Spendable,
		"equal updated_at MUST NOT trigger update (boundary)")
	require.NotNil(t, got.SpentBy)
	require.Equal(t, spentBy, *got.SpentBy)
}

// TestBRC40Guard_Flow_Regression mirrors sync.brc40.flow.regression.1:
// newer chunk arrives, then stale chunk for same row. Final state must reflect newer one.
func TestBRC40Guard_Flow_Regression(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	tNewer := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)
	tStale := time.Date(2026, 4, 23, 12, 30, 0, 0, time.UTC)

	reference := "ref-brc40-flow"
	txID := "1111222233334444555566667777888899990000aaaabbbbccccddddeeeeffff"

	// Chunk 1: newer
	_, _, err = repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt: tNewer, UpdatedAt: tNewer,
		UserID: user.ID, Status: wdk.TxStatusCompleted, Reference: reference,
		IsOutgoing: true, Satoshis: 1, TxID: &txID, Description: "newer",
	})
	require.NoError(t, err)

	// Chunk 2: stale
	_, _, err = repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt: tNewer, UpdatedAt: tStale,
		UserID: user.ID, Status: wdk.TxStatusUnsigned, Reference: reference,
		IsOutgoing: true, Satoshis: 1, Description: "stale",
	})
	require.NoError(t, err)

	var got models.Transaction
	require.NoError(t, d.DB.Where("userId = ? AND reference = ?", user.ID, reference).First(&got).Error)
	require.Equal(t, wdk.TxStatusCompleted, got.Status,
		"after stale-after-newer replay, final state MUST reflect the newer write (sync.brc40.flow.regression.1)")
	require.NotNil(t, got.TxID)
	require.Equal(t, txID, *got.TxID)
}
