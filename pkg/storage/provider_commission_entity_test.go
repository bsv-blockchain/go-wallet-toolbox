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
)

const (
	seedCommissionIterations = 10
	seededCommissionsCount   = 2 * seedCommissionIterations
)

func TestCommissionsCount(t *testing.T) {
	tests := map[string]struct {
		reader func(reader crud.CommissionReader)
		count  int64
	}{
		"get all rows": {
			count: seededCommissionsCount,
		},
		"get by id": {
			reader: func(reader crud.CommissionReader) {
				reader.ID(1)
			},
			count: 1,
		},
		"filter Alice's commissions": {
			reader: func(reader crud.CommissionReader) {
				reader.UserID(testusers.Alice.ID)
			},
			count: seedCommissionIterations,
		},
		"filter by Satoshis: equals": {
			reader: func(reader crud.CommissionReader) {
				reader.Satoshis().Equals(1000)
			},
			count: 2,
		},
		"filter by Satoshis: greater than": {
			reader: func(reader crud.CommissionReader) {
				reader.Satoshis().GreaterThan(1000)
			},
			count: seededCommissionsCount - 2,
		},
		"filter by Satoshis: greater than or equal": {
			reader: func(reader crud.CommissionReader) {
				reader.Satoshis().GreaterThanOrEqual(1000)
			},
			count: seededCommissionsCount,
		},
		"filter by Satoshis: less than": {
			reader: func(reader crud.CommissionReader) {
				reader.Satoshis().LessThan(1000)
			},
			count: 0,
		},
		"filter by Satoshis: less than or equal": {
			reader: func(reader crud.CommissionReader) {
				reader.Satoshis().LessThanOrEqual(1000)
			},
			count: 2,
		},
		"filter by Satoshis: between": {
			reader: func(reader crud.CommissionReader) {
				reader.Satoshis().Between(1000, 1005)
			},
			count: 12,
		},
		"filter by Satoshis: not between": {
			reader: func(reader crud.CommissionReader) {
				reader.Satoshis().NotBetween(1000, 1005)
			},
			count: seededCommissionsCount - 12,
		},
		"filter by Satoshis: between when values are wrongly swapped": {
			reader: func(reader crud.CommissionReader) {
				reader.Satoshis().Between(1005, 1000)
			},
			count: 12,
		},
		"filter by transaction id: equals": {
			reader: func(reader crud.CommissionReader) {
				reader.TransactionID().Equals(1)
			},
			count: 2,
		},
		"filter by transaction id: not equal": {
			reader: func(reader crud.CommissionReader) {
				reader.TransactionID().NotEquals(1)
			},
			count: seededCommissionsCount - 2,
		},
		"filter by transaction id: in": {
			reader: func(reader crud.CommissionReader) {
				reader.TransactionID().In(1, 2, 3)
			},
			count: 6,
		},
		"filter by transaction id: not in": {
			reader: func(reader crud.CommissionReader) {
				reader.TransactionID().NotIn(1, 2, 3)
			},
			count: seededCommissionsCount - 6,
		},
		"filter by key offset: equals": {
			reader: func(reader crud.CommissionReader) {
				reader.KeyOffset().Equals("key_offset_0")
			},
			count: 2,
		},
		"filter by key offset: not equal": {
			reader: func(reader crud.CommissionReader) {
				reader.KeyOffset().NotEquals("key_offset_0")
			},
			count: seededCommissionsCount - 2,
		},
		"filter by key offset: in": {
			reader: func(reader crud.CommissionReader) {
				reader.KeyOffset().In("key_offset_0", "key_offset_1", "key_offset_2")
			},
			count: 6,
		},
		"filter by key offset: not in": {
			reader: func(reader crud.CommissionReader) {
				reader.KeyOffset().NotIn("key_offset_0", "key_offset_1", "key_offset_2")
			},
			count: seededCommissionsCount - 6,
		},
		"filter by key offset: like to get all": {
			reader: func(reader crud.CommissionReader) {
				reader.KeyOffset().Like("%offset%")
			},
			count: seededCommissionsCount,
		},
		"filter by key offset: like to get one": {
			reader: func(reader crud.CommissionReader) {
				reader.UserID(testusers.Alice.ID).KeyOffset().Like("___________0")
			},
			count: 1,
		},
		"filter by key offset: not like": {
			reader: func(reader crud.CommissionReader) {
				reader.KeyOffset().NotLike("%offset%")
			},
			count: 0,
		},
		"since as now": {
			reader: func(reader crud.CommissionReader) {
				time.Sleep(10 * time.Millisecond) // Ensure none of the commissions are created at the exact same time
				reader.Since(time.Now(), entity.SinceFieldCreatedAt)
			},
			count: 0,
		},
		"since as 1 hour ago": {
			reader: func(reader crud.CommissionReader) {
				reader.Since(time.Now().Add(-time.Hour), entity.SinceFieldCreatedAt)
			},
			count: seededCommissionsCount,
		},
		"filter by IsRedeemed: true": {
			reader: func(reader crud.CommissionReader) {
				reader.IsRedeemed(true)
			},
			count: 0,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			activeStorage := seedDbWithCommissions(t)

			reader := activeStorage.CommissionEntity().Read()

			if test.reader != nil {
				test.reader(reader)
			}

			count, err := reader.Count(t.Context())
			require.NoError(t, err)
			assert.Equal(t, test.count, count, "expected count to be %d, got %d", test.count, count)
		})
	}
}

func TestCommissionUpdate(t *testing.T) {
	activeStorage := seedDbWithCommissions(t)

	// when:
	err := activeStorage.CommissionEntity().Update(t.Context(), &entity.CommissionUpdateSpecification{
		ID:         1,
		IsRedeemed: to.Ptr(true),
	})

	// then:
	require.NoError(t, err)

	// when:
	notRedeemedCount, err := activeStorage.CommissionEntity().Read().IsRedeemed(true).Count(t.Context())
	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(1), notRedeemedCount)
}

func TestCommissionFind(t *testing.T) {
	// given:
	activeStorage := seedDbWithCommissions(t)

	// when:
	commissions, err := activeStorage.CommissionEntity().Read().ID(1).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, commissions, 1)
	assert.Equal(t, uint(1), commissions[0].ID)
	assert.Equal(t, testusers.Alice.ID, commissions[0].UserID)
	assert.Equal(t, "key_offset_0", commissions[0].KeyOffset)
	assert.False(t, commissions[0].IsRedeemed)
}

func TestCommissionPagedFind(t *testing.T) {
	// given:
	activeStorage := seedDbWithCommissions(t)

	// when:
	commissionsPaged, err := activeStorage.CommissionEntity().Read().Paged(5, 5, false).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, commissionsPaged, 5)
	assert.Equal(t, uint(6), commissionsPaged[0].ID)
	assert.Equal(t, uint(10), commissionsPaged[4].ID)
}

func seedDbWithCommissions(t testing.TB) *storage.Provider {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	for i := range seedCommissionIterations {
		newCommission := &entity.Commission{
			UserID:        testusers.Alice.ID,
			TransactionID: uint(i),
			Satoshis:      1000 + uint64(i),
			KeyOffset:     fmt.Sprintf("key_offset_%d", i),
			IsRedeemed:    false,
			LockingScript: []byte(fmt.Sprintf("locking_script_%d", i)),
		}

		// when:
		err := activeStorage.CommissionEntity().Create(t.Context(), newCommission)

		// then:
		require.NoError(t, err)

		// when:
		newCommission.UserID = testusers.Bob.ID
		err = activeStorage.CommissionEntity().Create(t.Context(), newCommission)

		// then:
		require.NoError(t, err)
	}

	return activeStorage
}
