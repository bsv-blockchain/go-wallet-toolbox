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

// Tests for go-wallet-toolbox#851: ProvenTxReq.notify must round-trip through
// UpsertKnownTxForSync / FindKnownTxsForSync as an opaque blob.

func TestKnownTxNotify_UpsertPersistsOpaquePayload(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	t0 := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	// 64 hex chars to satisfy varchar(64) primary key on Postgres.
	txID := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	notifyPayload := `{"transactionIds":[42,99],"origin":"js-wallet"}`

	isNew, _, _, err := repos.UpsertKnownTxForSync(t.Context(), &entity.KnownTx{
		CreatedAt: t0,
		UpdatedAt: t0,
		TxID:      txID,
		Status:    wdk.ProvenTxStatusUnsent,
		RawTx:     []byte{0x01, 0x00},
		Notify:    notifyPayload,
	})
	require.NoError(t, err)
	require.True(t, isNew)

	var got models.KnownTx
	require.NoError(t, d.DB.First(&got, "tx_id = ?", txID).Error)
	require.Equal(t, notifyPayload, got.Notify,
		"incoming notify payload must be stored unchanged")
}

func TestKnownTxNotify_UpsertDefaultEmptyToObject(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	t0 := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	txID := "11223344556677889900aabbccddeeff11223344556677889900aabbccddeeff"

	_, _, err := repos.UpsertKnownTxForSync(t.Context(), &entity.KnownTx{
		CreatedAt: t0,
		UpdatedAt: t0,
		TxID:      txID,
		Status:    wdk.ProvenTxStatusUnsent,
		RawTx:     []byte{0x01},
		// Notify intentionally omitted / empty
	})
	require.NoError(t, err)

	var got models.KnownTx
	require.NoError(t, d.DB.First(&got, "tx_id = ?", txID).Error)
	require.Equal(t, "{}", got.Notify, "empty notify must default to {}")
}

func TestKnownTxNotify_FindForSyncRoundTrip(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	t0 := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	txID := "99887766554433221100ffeeddccbbaa99887766554433221100ffeeddccbbaa"
	notifyPayload := `{"transactionIds":[7],"note":"proof-pending"}`
	rawTx := []byte{0x02, 0x00, 0x00, 0x00}

	// Link a user transaction so FindKnownTxsForSync's EXISTS filter matches.
	_, _, err = repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt:   t0,
		UpdatedAt:   t0,
		UserID:      user.ID,
		Status:      wdk.TxStatusUnproven,
		Reference:   "ref-notify-roundtrip",
		IsOutgoing:  true,
		Satoshis:    100,
		Description: "notify round-trip",
		TxID:        &txID,
	})
	require.NoError(t, err)

	_, _, err = repos.UpsertKnownTxForSync(t.Context(), &entity.KnownTx{
		CreatedAt: t0,
		UpdatedAt: t0,
		TxID:      txID,
		Status:    wdk.ProvenTxStatusUnsent,
		RawTx:     rawTx,
		Notify:    notifyPayload,
	})
	require.NoError(t, err)

	reqs, proven, err := repos.FindKnownTxsForSync(t.Context(), user.ID)
	require.NoError(t, err)
	require.Empty(t, proven, "unmined known tx must appear as ProvenTxReq, not ProvenTx")
	require.Len(t, reqs, 1)
	require.Equal(t, txID, reqs[0].TxID)
	require.Equal(t, notifyPayload, reqs[0].Notify,
		"FindKnownTxsForSync must return the stored notify payload unchanged")
}

func TestKnownTxNotify_UpdateReplacesPayload(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	tOld := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	tNew := time.Date(2026, 7, 18, 13, 0, 0, 0, time.UTC)
	txID := "ddeeff00112233445566778899aabbccddeeff00112233445566778899aabbcc"

	_, _, err := repos.UpsertKnownTxForSync(t.Context(), &entity.KnownTx{
		CreatedAt: tOld,
		UpdatedAt: tOld,
		TxID:      txID,
		Status:    wdk.ProvenTxStatusUnsent,
		RawTx:     []byte{0x01},
		Notify:    `{"transactionIds":[1]}`,
	})
	require.NoError(t, err)

	_, _, err = repos.UpsertKnownTxForSync(t.Context(), &entity.KnownTx{
		CreatedAt: tOld,
		UpdatedAt: tNew,
		TxID:      txID,
		Status:    wdk.ProvenTxStatusUnmined,
		RawTx:     []byte{0x01},
		Notify:    `{"transactionIds":[1,2,3]}`,
	})
	require.NoError(t, err)

	var got models.KnownTx
	require.NoError(t, d.DB.First(&got, "tx_id = ?", txID).Error)
	require.JSONEq(t, `{"transactionIds":[1,2,3]}`, got.Notify,
		"newer updated_at must replace notify payload")
	require.Equal(t, wdk.ProvenTxStatusUnmined, got.Status)
}
