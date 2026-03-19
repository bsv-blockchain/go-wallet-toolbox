package storage_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/crud"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestTransactionCountFilters(t *testing.T) {
	tests := map[string]struct {
		filter func(reader crud.TransactionReader)
		count  int64
	}{
		"all transactions": {
			count: 12,
		},
		"user Alice only": {
			filter: func(r crud.TransactionReader) { r.UserID().Equals(testusers.Alice.ID) },
			count:  5,
		},
		"filter by status unprocessed": {
			filter: func(r crud.TransactionReader) { r.Status().Equals(wdk.TxStatusUnprocessed) },
			count:  10,
		},
		"filter by status failed": {
			filter: func(r crud.TransactionReader) { r.Status().Equals(wdk.TxStatusFailed) },
			count:  2,
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
		"description contains jerry": {
			filter: func(r crud.TransactionReader) {
				r.DescriptionContains().In([]string{"test transaction from jerry"}...)
			},
			count: 2,
		},
		"description contains bob": {
			filter: func(r crud.TransactionReader) { r.DescriptionContains().Like("%bob%") },
			count:  5,
		},
		"since now": {
			filter: func(r crud.TransactionReader) {
				since := time.Now().Add(time.Minute) // delta time makes sure no "timing flakiness" happens during test execution
				r.Since(since, entity.SinceFieldCreatedAt)
			},
			count: 0,
		},
		"label contains alpha": {
			filter: func(r crud.TransactionReader) {
				r.Labels().ContainAny("alpha")
			},
			count: 5,
		},
		"label contains beta": {
			filter: func(r crud.TransactionReader) {
				r.Labels().ContainAny("beta")
			},
			count: 5,
		},
		"label contains gamma": {
			filter: func(r crud.TransactionReader) {
				r.Labels().ContainAny("gamma")
			},
			count: 5,
		},
		"label contains alpha AND beta (contain any)": {
			filter: func(r crud.TransactionReader) {
				r.Labels().ContainAny("alpha", "beta")
			},
			count: 10,
		},
		"label contains alpha AND gamma (contain any)": {
			filter: func(r crud.TransactionReader) {
				r.Labels().ContainAny("alpha", "gamma")
			},
			count: 10,
		},
		"label contains nothing (empty filter) for any": {
			filter: func(r crud.TransactionReader) {
				r.Labels().ContainAny()
			},
			count: 12,
		},
		"label must contain both alpha AND beta (contain all)": {
			filter: func(r crud.TransactionReader) {
				r.Labels().ContainAll("alpha", "beta")
			},
			count: 0,
		},
		"label must contain both beta AND gamma (contain all)": {
			filter: func(r crud.TransactionReader) {
				r.Labels().ContainAll("beta", "gamma")
			},
			count: 5,
		},
		"label contains nothing (empty filter) for all": {
			filter: func(r crud.TransactionReader) {
				r.Labels().ContainAll()
			},
			count: 12,
		},
		"label is empty": {
			filter: func(r crud.TransactionReader) {
				r.Labels().Empty()
			},
			count: 2,
		},
		"user Alice with empty labels": {
			filter: func(r crud.TransactionReader) {
				r.UserID().Equals(testusers.Alice.ID)
				r.Labels().Empty()
			},
			count: 0,
		},
		"user Bob with empty labels": {
			filter: func(r crud.TransactionReader) {
				r.UserID().Equals(testusers.Bob.ID)
				r.Labels().Empty()
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

func TestTransactionUpdateStatus(t *testing.T) {
	activeStorage := seedDbWithTransactions(t)

	// when:
	newStatus := wdk.TxStatusCompleted
	err := activeStorage.TransactionEntity().Update(t.Context(), &entity.TransactionUpdateSpecification{
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
	newStatus := wdk.TxStatusNoSend
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

	for i := range 5 {
		tx := &entity.Transaction{
			UserID:      testusers.Alice.ID,
			Status:      wdk.TxStatusUnprocessed,
			Reference:   fmt.Sprintf("ref_alice_%d", i),
			IsOutgoing:  true,
			Satoshis:    1000 + int64(i),
			Description: "test transaction from alice",
			Version:     1,
			LockTime:    0,
			TxID:        to.Ptr(fmt.Sprintf("txid_alice_%d", i)),
			Labels:      []string{"alpha"},
		}
		require.NoError(t, activeStorage.TransactionEntity().Create(t.Context(), tx))
	}

	for i := range 5 {
		tx := &entity.Transaction{
			UserID:      testusers.Bob.ID,
			Status:      wdk.TxStatusUnprocessed,
			Reference:   fmt.Sprintf("ref_bob_%d", i),
			IsOutgoing:  false,
			Satoshis:    1005 + int64(i),
			Description: "test transaction from bob",
			Version:     1,
			LockTime:    0,
			TxID:        to.Ptr(fmt.Sprintf("txid_bob_%d", i)),
			Labels:      []string{"beta", "gamma"},
		}
		require.NoError(t, activeStorage.TransactionEntity().Create(t.Context(), tx))
	}

	for i := range 2 {
		tx := &entity.Transaction{
			UserID:      777,
			Status:      wdk.TxStatusFailed,
			Reference:   fmt.Sprintf("ref_jerry_%d", i),
			IsOutgoing:  false,
			Satoshis:    1 + int64(i),
			Description: "test transaction from jerry",
			Version:     1,
			LockTime:    0,
			TxID:        to.Ptr(fmt.Sprintf("txid_jerry_%d", i)),
		}
		require.NoError(t, activeStorage.TransactionEntity().Create(t.Context(), tx))
	}

	return activeStorage
}
