package storage_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommission(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	const loopCount = 10
	for i := range loopCount {
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

	// when:
	count, err := activeStorage.CommissionEntity().Read().Count(t.Context())

	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(2*loopCount), count)

	// when:
	aliceCount, err := activeStorage.CommissionEntity().Read().
		UserID(testusers.Alice.ID).
		Count(t.Context())

	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(loopCount), aliceCount)

	// when:
	commissionID1, err := activeStorage.CommissionEntity().Read().ID(1).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, commissionID1, 1)
	assert.Equal(t, uint(1), commissionID1[0].ID)
	assert.Equal(t, testusers.Alice.ID, commissionID1[0].UserID)
	assert.Equal(t, "key_offset_0", commissionID1[0].KeyOffset)
	assert.Equal(t, false, commissionID1[0].IsRedeemed)

	// when:
	err = activeStorage.CommissionEntity().Update(t.Context(), &entity.CommissionUpdateSpecification{
		ID:         commissionID1[0].ID,
		IsRedeemed: to.Ptr(true),
	})

	// then:
	require.NoError(t, err)

	// when:
	notRedeemedCount, err := activeStorage.CommissionEntity().Read().IsRedeemed(false).Count(t.Context())
	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(2*loopCount-1), notRedeemedCount)

	// when:
	count, err = activeStorage.CommissionEntity().Read().Satoshis().Equals(1000).Count(t.Context())
	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// when:
	count, err = activeStorage.CommissionEntity().Read().Satoshis().GreaterThan(1000).Count(t.Context())
	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(2*loopCount-2), count)

	// when:
	count, err = activeStorage.CommissionEntity().Read().Satoshis().GreaterThanOrEqual(1000).Count(t.Context())
	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(2*loopCount), count)

	// when:
	count, err = activeStorage.CommissionEntity().Read().Satoshis().LessThan(1000).Count(t.Context())
	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// when:
	count, err = activeStorage.CommissionEntity().Read().Satoshis().LessThanOrEqual(1000).Count(t.Context())
	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// when:
	count, err = activeStorage.CommissionEntity().Read().Since(time.Now(), entity.SinceFieldCreatedAt).Count(t.Context())
	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// when:
	count, err = activeStorage.CommissionEntity().Read().Since(time.Now().Add(-time.Hour), entity.SinceFieldCreatedAt).Count(t.Context())
	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(2*loopCount), count)

	// when:
	paged, err := activeStorage.CommissionEntity().Read().Paged(5, 5, false).Find(t.Context())
	// then:
	require.NoError(t, err)
	require.Len(t, paged, 5)
	assert.Equal(t, uint(6), paged[0].ID)
	assert.Equal(t, uint(10), paged[4].ID)
}
