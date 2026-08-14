package repo_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// A DirectSourcesOnly build needs each subject's immediate parents and nothing
// above them. Where those parents are known txs in their own right it reads
// them directly, because the stored input beef carries the whole ancestry and
// is large. Where they are not - inputs the caller supplied exist only inside
// that blob - it still has to be merged, or the build loses them.

func beefTestTx(lockTime uint32, parents ...*transaction.Transaction) *transaction.Transaction {
	tx := &transaction.Transaction{Version: 1, LockTime: lockTime}
	for _, p := range parents {
		tx.Inputs = append(tx.Inputs, &transaction.TransactionInput{
			SourceTXID:        p.TxID(),
			SourceTransaction: p,
			SourceTxOutIndex:  0,
			SequenceNumber:    0xffffffff,
		})
	}
	tx.Outputs = append(tx.Outputs, &transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: &script.Script{},
	})
	return tx
}

func storeKnownTx(t *testing.T, db *database.Database, tx *transaction.Transaction, inputBEEF []byte) {
	t.Helper()
	require.NoError(t, db.DB.Create(&models.KnownTx{
		TxID:      tx.TxID().String(),
		Status:    wdk.ProvenTxStatusUnprocessed,
		RawTx:     tx.Bytes(),
		InputBeef: inputBEEF,
	}).Error)
}

func TestGetBEEFForTxIDs_DirectSourcesOnly_StorageKnownParents(t *testing.T) {
	// given: grandparent <- parent <- subject, all known to storage, with the
	// subject's stored input beef holding the whole ancestry.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	grandparent := beefTestTx(1)
	parent := beefTestTx(2, grandparent)
	subject := beefTestTx(3, parent)

	ancestry := transaction.NewBeefV2()
	_, err := ancestry.MergeTransaction(parent)
	require.NoError(t, err)
	ancestryBytes, err := ancestry.Bytes()
	require.NoError(t, err)

	storeKnownTx(t, db, grandparent, nil)
	storeKnownTx(t, db, parent, nil)
	storeKnownTx(t, db, subject, ancestryBytes)

	repos := repo.NewSQLRepositories(db.DB)

	// when:
	beef, err := repos.KnownTx.GetBEEFForTxIDs(
		t.Context(),
		seq.FromSlice([]string{subject.TxID().String()}),
		entity.WithDirectSourcesOnly(),
	)

	// then: the subject and its direct parent are present...
	require.NoError(t, err)
	require.NotNil(t, beef.FindTransaction(subject.TxID().String()), "subject must be in the BEEF")
	require.NotNil(t, beef.FindTransaction(parent.TxID().String()), "direct parent must be in the BEEF")

	// ...and nothing above them, so the stored ancestry blob was not merged.
	assert.Nil(t, beef.FindTransaction(grandparent.TxID().String()),
		"a direct-sources-only build must stop at the immediate parents")
}

func TestGetBEEFForTxIDs_DirectSourcesOnly_CallerSuppliedParent(t *testing.T) {
	// given: the subject spends a transaction that is NOT a known tx. It exists
	// only inside the subject's stored input beef.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	external := beefTestTx(11)
	subject := beefTestTx(12, external)

	ancestry := transaction.NewBeefV2()
	_, err := ancestry.MergeTransaction(external)
	require.NoError(t, err)
	ancestryBytes, err := ancestry.Bytes()
	require.NoError(t, err)

	storeKnownTx(t, db, subject, ancestryBytes)

	repos := repo.NewSQLRepositories(db.DB)

	// when:
	beef, err := repos.KnownTx.GetBEEFForTxIDs(
		t.Context(),
		seq.FromSlice([]string{subject.TxID().String()}),
		entity.WithDirectSourcesOnly(),
	)

	// then: the parent still arrives, via the stored input beef.
	require.NoError(t, err, "a caller-supplied parent must not be dropped")
	require.NotNil(t, beef.FindTransaction(subject.TxID().String()))
	assert.NotNil(t, beef.FindTransaction(external.TxID().String()),
		"the only copy of a caller-supplied parent lives in the stored input beef")
}
