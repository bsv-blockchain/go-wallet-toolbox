package storage_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/crud"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionCountFilters(t *testing.T) {
	tests := map[string]struct {
		filter func(reader crud.TransactionReader)
		count  int64
	}{
		"all transactions": {
			count: 10,
		},
		"user Alice only": {
			filter: func(r crud.TransactionReader) { r.UserID().Equals(testusers.Alice.ID) },
			count:  5,
		},
		"filter by status": {
			filter: func(r crud.TransactionReader) { r.Status().Equals(wdk.TxStatusUnprocessed) },
			count:  10,
		},
		"satoshis greater than": {
			filter: func(r crud.TransactionReader) { r.Satoshis().GreaterThan(1005) },
			count:  4,
		},
		"description contains": {
			filter: func(r crud.TransactionReader) {
				r.DescriptionContains().In([]string{"test transaction", "test transaction from alice", "test transaction from bob"}...)
			},
			count: 10,
		},
		"description contains bob": {
			filter: func(r crud.TransactionReader) { r.DescriptionContains().Like("%bob%") },
			count:  5,
		},
		"since now": {
			filter: func(r crud.TransactionReader) {
				since := time.Now().Add(time.Minute) // delta time makes sure no "timing flakiness" happens during test execution
				r.Since(since, pkgentity.SinceFieldCreatedAt)
			},
			count: 0,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			activeStorage := seedDbWithTransactions(t)

			// when:
			reader := activeStorage.TransactionEntity().Read()
			if test.filter != nil {
				test.filter(reader)
			}

			// then:
			count, err := reader.Count(t.Context())
			require.NoError(t, err)
			assert.Equal(t, test.count, count)
		})
	}
}

func TestTransactionByLabels(t *testing.T) {
	activeStorage := seedDbWithTransactions(t)

	txs, err := activeStorage.TransactionEntity().Read().
		Labels().ContainAny("a").
		Find(t.Context())
	require.NoError(t, err)
	require.Len(t, txs, 9)
}

func TestTransactionUpdateStatus(t *testing.T) {
	activeStorage := seedDbWithTransactions(t)

	// when:
	newStatus := wdk.TxStatusCompleted
	err := activeStorage.TransactionEntity().Update(t.Context(), &pkgentity.TransactionUpdateSpecification{
		ID:     1,
		Status: &newStatus,
	})

	// then:
	require.NoError(t, err)

	// when:
	count, err := activeStorage.TransactionEntity().Read().Status().Equals(newStatus).Count(t.Context())

	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestTransactionUpdateDescription(t *testing.T) {
	activeStorage := seedDbWithTransactions(t)

	// given:
	newDescription := "updated description"
	err := activeStorage.TransactionEntity().Update(t.Context(), &entity.TransactionUpdateSpecification{
		ID:          2,
		Description: &newDescription,
	})
	require.NoError(t, err)

	// when:
	transactions, err := activeStorage.TransactionEntity().Read().ID(2).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, transactions, 1)
	assert.Equal(t, "updated description", transactions[0].Description)
}

func TestTransactionUpdateStatusAndDescription(t *testing.T) {
	activeStorage := seedDbWithTransactions(t)
	// given:
	newStatus := wdk.TxStatusFailed
	newDescription := "failed due to timeout"
	err := activeStorage.TransactionEntity().Update(t.Context(), &entity.TransactionUpdateSpecification{
		ID:          3,
		Status:      &newStatus,
		Description: &newDescription,
	})
	require.NoError(t, err)

	// when:
	tx, err := activeStorage.TransactionEntity().Read().Status().Equals(newStatus).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, tx, 1)
	assert.Equal(t, "failed due to timeout", tx[0].Description)
}

func TestTransactionFind(t *testing.T) {
	// given:
	activeStorage := seedDbWithTransactions(t)

	// when:
	txs, err := activeStorage.TransactionEntity().Read().ID(1).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, txs, 1)
	assert.Equal(t, uint(1), txs[0].ID)
	assert.Equal(t, testusers.Alice.ID, txs[0].UserID)
	assert.Equal(t, "ref_alice_0", txs[0].Reference)
	assert.Equal(t, "test transaction from alice", txs[0].Description)
}

func TestTransactionPagedFind(t *testing.T) {
	// given:
	activeStorage := seedDbWithTransactions(t)

	// when:
	txsPaged, err := activeStorage.TransactionEntity().Read().Paged(5, 5, false).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, txsPaged, 5)
	assert.Equal(t, uint(6), txsPaged[0].ID)
	assert.Equal(t, uint(10), txsPaged[4].ID)
}

func seedDbWithTransactions(t testing.TB) *storage.Provider {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	var labels []string
	nextLabel := byte('a')

	for i := range 5 {
		tx := &pkgentity.Transaction{
			UserID:      testusers.Alice.ID,
			Status:      wdk.TxStatusUnprocessed,
			Reference:   fmt.Sprintf("ref_alice_%d", i),
			IsOutgoing:  true,
			Satoshis:    1000 + int64(i),
			Description: "test transaction from alice",
			Version:     1,
			LockTime:    0,
			TxID:        to.Ptr(fmt.Sprintf("txid_alice_%d", i)),
			Labels:      slices.Clone(labels),
		}
		require.NoError(t, activeStorage.TransactionEntity().Create(t.Context(), tx))

		labels = append(labels, fmt.Sprintf("%c", nextLabel))
		nextLabel += 1
	}

	for i := range 5 {
		tx := &pkgentity.Transaction{
			UserID:      testusers.Bob.ID,
			Status:      wdk.TxStatusUnprocessed,
			Reference:   fmt.Sprintf("ref_bob_%d", i),
			IsOutgoing:  false,
			Satoshis:    1005 + int64(i),
			Description: "test transaction from bob",
			Version:     1,
			LockTime:    0,
			TxID:        to.Ptr(fmt.Sprintf("txid_bob_%d", i)),
			Labels:      slices.Clone(labels),
		}
		require.NoError(t, activeStorage.TransactionEntity().Create(t.Context(), tx))

		labels = append(labels, fmt.Sprintf("%c", nextLabel))
		nextLabel += 1
	}

	return activeStorage
}
