package testabilities

import (
	"context"
	"maps"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type StorageReader interface {
	FindKnownTx(ctx context.Context, txID string) (*entity.KnownTx, error)
}

type DBStateAssertion interface {
	HasKnownTXs(txIDs ...string) DBStateAssertion
	HasKnownTX(txID string) KnownTxAssertion
}

type KnownTxAssertion interface {
	WithStatus(state wdk.ProvenTxReqStatus) KnownTxAssertion
	IsMined() KnownTxAssertion
	NotMined() KnownTxAssertion
	HasRawTx() KnownTxAssertion
}

type UserTransactionAssertion interface {
	WithStatus(state wdk.TxStatus) UserTransactionAssertion
	WithProvenTxID(provenTxID string) UserTransactionAssertion
	WithoutProvenTxID() UserTransactionAssertion
}

func ThenDBState(t testing.TB, storage StorageReader) DBStateAssertion {
	t.Helper()

	if storage == nil {
		require.FailNow(t, "Storage cannot be nil")
	}

	return &dbStateAssertion{
		TB:      t,
		storage: storage,
	}
}

type dbStateAssertion struct {
	testing.TB
	storage StorageReader
}

func (d *dbStateAssertion) HasKnownTXs(txIDs ...string) DBStateAssertion {
	d.Helper()

	missingTXs := map[string]struct{}{}

	for _, txID := range txIDs {
		knownTx, err := d.storage.FindKnownTx(d.Context(), txID)
		require.NoError(d.TB, err, txID)

		if knownTx == nil {
			missingTXs[txID] = struct{}{}
		}
	}

	if len(missingTXs) != 0 {
		missingIDs := seq.Collect(maps.Keys(missingTXs))
		assert.Failf(d, "Expected to find all the transactions", "missing transaction IDs: %v", missingIDs)
	}

	return d
}

func (d *dbStateAssertion) HasKnownTX(txID string) KnownTxAssertion {
	d.Helper()

	knownTx, err := d.storage.FindKnownTx(d.Context(), txID)
	require.NoError(d.TB, err, txID)

	if knownTx == nil {
		require.Failf(d, "Expected to find the transaction", "transaction ID: %s", txID)
		return nil
	}

	assert.Equal(d, txID, knownTx.TxID, "Expected known transaction to have the same TxID as the one requested")

	return &knownTxAssertion{
		TB:      d.TB,
		knownTx: knownTx,
	}
}

//func (d *dbStateAssertion) HasUserTransaction(txID string) UserTransactionAssertion {
//	d.Helper()
//
//	tx, err := d.storage
//	require.NoError(d.TB, err, txID)
//
//	if tx == nil {
//		require.Failf(d, "Expected to find the transaction", "transaction ID: %s", txID)
//		return nil
//	}
//
//	assert.Equal(d, txID, tx.TxID, "Expected user transaction to have the same TxID as the one requested")
//
//	return &userTransactionAssertion{
//		TB:      d.TB,
//		knownTx: tx,
//	}
//}

type knownTxAssertion struct {
	testing.TB
	knownTx *entity.KnownTx
}

func (d *knownTxAssertion) WithStatus(state wdk.ProvenTxReqStatus) KnownTxAssertion {
	d.Helper()
	assert.Equal(d, state, d.knownTx.Status, "Expected known transaction to have the status %s", state)
	return d
}

func (d *knownTxAssertion) IsMined() KnownTxAssertion {
	d.Helper()
	assert.NotNil(d, d.knownTx.BlockHeight)
	assert.NotEmpty(d, d.knownTx.MerklePath)
	assert.NotEmpty(d, d.knownTx.MerkleRoot)
	assert.NotEmpty(d, d.knownTx.BlockHash)
	return d
}

func (d *knownTxAssertion) NotMined() KnownTxAssertion {
	d.Helper()
	assert.Nil(d, d.knownTx.BlockHeight)
	assert.Empty(d, d.knownTx.MerklePath)
	assert.Empty(d, d.knownTx.MerkleRoot)
	assert.Empty(d, d.knownTx.BlockHash)
	assert.NotEqual(d, d.knownTx.Status, wdk.ProvenTxStatusCompleted)
	return d
}

func (d *knownTxAssertion) HasRawTx() KnownTxAssertion {
	d.Helper()
	assert.NotEmpty(d, d.knownTx.RawTx, "Expected known transaction to have a non-empty RawTx")
	return d
}
