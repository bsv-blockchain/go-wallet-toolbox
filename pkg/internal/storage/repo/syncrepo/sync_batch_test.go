package syncrepo_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type syncDump struct {
	KnownTxs     []models.KnownTx
	Notes        []models.TxNote
	Transactions []models.Transaction
	Outputs      []models.Output
	UserUTXOs    []models.UserUTXO
	NumericIDs   []models.NumericIDLookup
}

func dumpSyncTables(t *testing.T, db *gorm.DB) syncDump {
	t.Helper()
	var d syncDump
	require.NoError(t, db.Order("tx_id").Find(&d.KnownTxs).Error)
	require.NoError(t, db.Order("tx_id, what, id").Find(&d.Notes).Error)
	require.NoError(t, db.Order("reference").Find(&d.Transactions).Error)
	require.NoError(t, db.Order("transaction_id, vout").Find(&d.Outputs).Error)
	require.NoError(t, db.Order("output_id").Find(&d.UserUTXOs).Error)
	require.NoError(t, db.Order("string_id").Find(&d.NumericIDs).Error)

	zero := time.Time{}
	for i := range d.KnownTxs {
		d.KnownTxs[i].CreatedAt, d.KnownTxs[i].UpdatedAt = zero, zero
	}
	for i := range d.Notes {
		d.Notes[i].ID, d.Notes[i].CreatedAt = 0, zero
	}
	for i := range d.Transactions {
		d.Transactions[i].CreatedAt, d.Transactions[i].UpdatedAt = zero, zero
	}
	for i := range d.Outputs {
		d.Outputs[i].CreatedAt, d.Outputs[i].UpdatedAt = zero, zero
	}
	for i := range d.UserUTXOs {
		d.UserUTXOs[i].CreatedAt = zero
	}

	return d
}

type syncBatchOutcome struct {
	KnownTxs     []bool
	Transactions []bool
	Outputs      []bool
	Dump         syncDump
}

// runSyncBatchScenario seeds some rows through the single-row methods and then upserts a mix of new,
// newer and stale entities through the slice methods.
func runSyncBatchScenario(t *testing.T, batchSize int) syncBatchOutcome {
	t.Helper()
	d, cleanup := dbfixtures.TestDatabase(t, func(c *defs.Database) { c.InsertBatchSize = batchSize })
	defer cleanup()
	repos := d.CreateRepositories()
	ctx := t.Context()

	user, err := repos.CreateUser(ctx, testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: "default", NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000})
	require.NoError(t, err)

	t1 := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	note := func(what string) []*entity.TxHistoryNote {
		return []*entity.TxHistoryNote{{HistoryNote: wdk.HistoryNote{When: t1, What: what}}}
	}

	// known txs: ids 0,1 pre-exist at t1; the batch carries 0 newer, 1 stale, 2..6 new
	knownReq := func(i int, at time.Time, status wdk.ProvenTxReqStatus) *entity.KnownTx {
		return &entity.KnownTx{
			CreatedAt: at, UpdatedAt: at, TxID: fmt.Sprintf("%064x", i), Status: status,
			RawTx: []byte{byte(i)}, TxNotes: note(fmt.Sprintf("n%d-%s", i, status)),
		}
	}
	for i := range 2 {
		_, _, err = repos.UpsertKnownTxForSync(ctx, knownReq(i, t1, wdk.ProvenTxStatusUnsent))
		require.NoError(t, err)
	}
	knownBatch := []*entity.KnownTx{knownReq(0, t2, wdk.ProvenTxStatusCompleted), knownReq(1, t1, wdk.ProvenTxStatusInvalid)}
	for i := 2; i < 7; i++ {
		knownBatch = append(knownBatch, knownReq(i, t1, wdk.ProvenTxStatusUnsent))
	}
	knownRes, err := repos.UpsertKnownTxsForSync(ctx, knownBatch)
	require.NoError(t, err)
	require.Len(t, knownRes, len(knownBatch))

	// transactions: ref-0 pre-exists; batch carries 0 newer, 1 stale-equal, 2..6 new
	txEntity := func(i int, at time.Time, status wdk.TxStatus) *entity.Transaction {
		return &entity.Transaction{
			CreatedAt: t1, UpdatedAt: at, UserID: user.ID, Status: status,
			Reference: fmt.Sprintf("ref-%d", i), Satoshis: int64(i), Description: fmt.Sprintf("d%d", i),
		}
	}
	for i := range 2 {
		_, _, err = repos.UpsertTransactionForSync(ctx, txEntity(i, t1, wdk.TxStatusUnsigned))
		require.NoError(t, err)
	}
	txBatch := []*entity.Transaction{txEntity(0, t2, wdk.TxStatusCompleted), txEntity(1, t1, wdk.TxStatusFailed)}
	for i := 2; i < 7; i++ {
		txBatch = append(txBatch, txEntity(i, t1, wdk.TxStatusCompleted))
	}
	txRes, err := repos.UpsertTransactionsForSync(ctx, txBatch)
	require.NoError(t, err)
	require.Len(t, txRes, len(txBatch))

	// outputs on transactions 2..6 (completed -> user utxos get created)
	basket := "default"
	outEntity := func(txID uint, vout uint32, at time.Time, sats int64) *entity.Output {
		return &entity.Output{
			CreatedAt: t1, UpdatedAt: at, UserID: user.ID, TransactionID: txID, Vout: vout, Satoshis: sats,
			BasketName: &basket, Spendable: true, Change: true, ProvidedBy: "storage", Purpose: "change", Type: "P2PKH",
			UserUTXO: &entity.UserUTXO{UserID: user.ID, BasketName: basket, Satoshis: uint64(sats), EstimatedInputSize: 148, CreatedAt: t1},
		}
	}
	_, _, err = repos.UpsertOutputForSync(ctx, outEntity(txRes[2].TransactionID, 0, t1, 100))
	require.NoError(t, err)
	var outBatch []*entity.Output
	outBatch = append(outBatch, outEntity(txRes[2].TransactionID, 0, t2, 111)) // newer update of the existing one
	for i := 3; i < 7; i++ {
		outBatch = append(outBatch, outEntity(txRes[i].TransactionID, 0, t1, int64(100+i)))
		outBatch = append(outBatch, outEntity(txRes[i].TransactionID, 1, t1, int64(200+i)))
	}
	outRes, err := repos.UpsertOutputsForSync(ctx, outBatch)
	require.NoError(t, err)
	require.Len(t, outRes, len(outBatch))

	// duplicate keys inside one batch keep the sequential semantics
	dupRes, err := repos.UpsertKnownTxsForSync(ctx, []*entity.KnownTx{
		knownReq(7, t1, wdk.ProvenTxStatusUnsent), knownReq(7, t2, wdk.ProvenTxStatusCompleted),
	})
	require.NoError(t, err)
	assert.True(t, dupRes[0].IsNew)
	assert.False(t, dupRes[1].IsNew)
	assert.Equal(t, dupRes[0].KnownTxNumID, dupRes[1].KnownTxNumID)

	out := syncBatchOutcome{Dump: dumpSyncTables(t, d.DB)}
	for _, r := range knownRes {
		out.KnownTxs = append(out.KnownTxs, r.IsNew)
	}
	for _, r := range txRes {
		out.Transactions = append(out.Transactions, r.IsNew)
	}
	for _, r := range outRes {
		out.Outputs = append(out.Outputs, r.IsNew)
	}

	return out
}

func TestSyncBatchUpserts_BatchedMatchesSingleRow(t *testing.T) {
	single := runSyncBatchScenario(t, 0)
	// batch sizes smaller than the input force several statements per kind
	for _, size := range []int{2, 3, 100} {
		t.Run(fmt.Sprintf("batch_size_%d", size), func(t *testing.T) {
			batched := runSyncBatchScenario(t, size)
			assert.Equal(t, single.KnownTxs, batched.KnownTxs)
			assert.Equal(t, single.Transactions, batched.Transactions)
			assert.Equal(t, single.Outputs, batched.Outputs)
			assert.Equal(t, single.Dump, batched.Dump)
		})
	}

	// sanity: the scenario really exercised create, update and skip
	assert.Equal(t, []bool{false, false, true, true, true, true, true}, single.KnownTxs)
	assert.Len(t, single.Dump.Outputs, 9)
	assert.Len(t, single.Dump.UserUTXOs, 9)
}
