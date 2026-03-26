package storage_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestUserUTXOCreateAndFindByUserID(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	provider := given.Provider().GORM()

	utxo := &entity.UserUTXO{
		UserID:             42,
		OutputID:           101,
		BasketName:         "basket-A",
		Satoshis:           12345,
		EstimatedInputSize: 148,
		CreatedAt:          time.Now(),
		Status:             wdk.UTXOStatusMined,
	}

	// when:
	err := provider.UserUTXOEntity().Create(t.Context(), utxo)

	// then:
	require.NoError(t, err)

	// when:
	found, err := provider.UserUTXOEntity().
		Read().
		UserID(42).
		Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, utxo.OutputID, found[0].OutputID)
	assert.Equal(t, utxo.BasketName, found[0].BasketName)
}

func TestUserUTXOCountByStatus(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	provider := given.Provider().GORM()

	utxo := &entity.UserUTXO{
		UserID:             55,
		OutputID:           200,
		BasketName:         "basket-B",
		Satoshis:           54321,
		EstimatedInputSize: 150,
		CreatedAt:          time.Now(),
		Status:             wdk.UTXOStatusSending,
	}
	require.NoError(t, provider.UserUTXOEntity().Create(t.Context(), utxo))

	// when:
	count, err := provider.UserUTXOEntity().
		Read().
		Status().Equals(wdk.UTXOStatusSending).
		Count(t.Context())

	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestUserUTXOUpdateStatus(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	provider := given.Provider().GORM()

	utxo := &entity.UserUTXO{
		UserID:             77,
		OutputID:           300,
		BasketName:         "basket-C",
		Satoshis:           99999,
		EstimatedInputSize: 150,
		CreatedAt:          time.Now(),
		Status:             wdk.UTXOStatusUnproven,
	}
	require.NoError(t, provider.UserUTXOEntity().Create(t.Context(), utxo))

	// when:
	newStatus := wdk.UTXOStatusMined
	err := provider.UserUTXOEntity().Update(t.Context(), &entity.UserUTXOUpdateSpecification{
		OutputID: utxo.OutputID,
		Status:   &newStatus,
	})

	// then:
	require.NoError(t, err)

	// when:
	found, err := provider.UserUTXOEntity().
		Read().
		OutputID().Equals(utxo.OutputID).
		Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, wdk.UTXOStatusMined, found[0].Status)
}

func TestUserUTXOFindByBasketAndPaged(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	provider := given.Provider().GORM()

	for i := 0; i < 10; i++ {
		utxo := &entity.UserUTXO{
			UserID:             99,
			OutputID:           uint(i + 1),
			BasketName:         "basket-Z",
			Satoshis:           uint64(1000 + i),
			EstimatedInputSize: 150,
			CreatedAt:          time.Now(),
			Status:             wdk.UTXOStatusMined,
		}
		require.NoError(t, provider.UserUTXOEntity().Create(t.Context(), utxo))
	}

	// when:
	result, err := provider.UserUTXOEntity().
		Read().
		BasketName().Equals("basket-Z").
		Paged(5, 5, false).
		Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, result, 5)
	assert.Equal(t, uint(6), result[0].OutputID)
}

func TestUserUTXOUpdateReservedByID(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	provider := given.Provider().GORM()

	utxo := &entity.UserUTXO{
		UserID:             101,
		OutputID:           500,
		BasketName:         "basket-R",
		Satoshis:           75000,
		EstimatedInputSize: 160,
		CreatedAt:          time.Now(),
		Status:             wdk.UTXOStatusUnproven,
	}
	require.NoError(t, provider.UserUTXOEntity().Create(t.Context(), utxo))

	// when:
	reservedBy := uint(9001)
	err := provider.UserUTXOEntity().Update(t.Context(), &entity.UserUTXOUpdateSpecification{
		OutputID:     utxo.OutputID,
		ReservedByID: &reservedBy,
	})

	// then:
	require.NoError(t, err)

	// when:
	found, err := provider.UserUTXOEntity().
		Read().
		OutputID().Equals(utxo.OutputID).
		Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.NotNil(t, found[0].ReservedByID)
	assert.Equal(t, reservedBy, *found[0].ReservedByID)
}

func TestUserUTXOSinceCreatedAt(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	provider := given.Provider().GORM()

	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-5 * time.Minute)

	require.NoError(t, provider.UserUTXOEntity().Create(t.Context(), &entity.UserUTXO{
		UserID:             303,
		OutputID:           401,
		BasketName:         "time",
		Satoshis:           1000,
		EstimatedInputSize: 148,
		CreatedAt:          oldTime,
		Status:             wdk.UTXOStatusMined,
	}))
	require.NoError(t, provider.UserUTXOEntity().Create(t.Context(), &entity.UserUTXO{
		UserID:             303,
		OutputID:           402,
		BasketName:         "time",
		Satoshis:           2000,
		EstimatedInputSize: 148,
		CreatedAt:          newTime,
		Status:             wdk.UTXOStatusMined,
	}))

	// when:
	since := time.Now().Add(-1 * time.Hour)
	result, err := provider.UserUTXOEntity().
		Read().
		Since(since, entity.SinceFieldCreatedAt).
		UserID(303).
		Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, uint(402), result[0].OutputID)
}

func TestUserUTXOFindByUserIDAndStatus(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	provider := given.Provider().GORM()

	require.NoError(t, provider.UserUTXOEntity().Create(t.Context(), &entity.UserUTXO{
		UserID:             202,
		OutputID:           301,
		BasketName:         "compound",
		Satoshis:           10000,
		EstimatedInputSize: 150,
		CreatedAt:          time.Now(),
		Status:             wdk.UTXOStatusMined,
	}))

	require.NoError(t, provider.UserUTXOEntity().Create(t.Context(), &entity.UserUTXO{
		UserID:             202,
		OutputID:           302,
		BasketName:         "compound",
		Satoshis:           10000,
		EstimatedInputSize: 150,
		CreatedAt:          time.Now(),
		Status:             wdk.UTXOStatusUnproven,
	}))

	// when:
	result, err := provider.UserUTXOEntity().
		Read().
		Status().Equals(wdk.UTXOStatusMined).
		UserID(202).
		Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, uint(301), result[0].OutputID)
}

func TestUserUTXOFindBySatoshis(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	provider := given.Provider().GORM()

	utxo := &entity.UserUTXO{
		UserID:             111,
		OutputID:           601,
		BasketName:         "basket-S",
		Satoshis:           88000,
		EstimatedInputSize: 150,
		CreatedAt:          time.Now(),
		Status:             wdk.UTXOStatusMined,
	}
	require.NoError(t, provider.UserUTXOEntity().Create(t.Context(), utxo))

	// when:
	result, err := provider.UserUTXOEntity().
		Read().
		UserID(111).
		Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, uint64(88000), result[0].Satoshis)
}

func TestUserUTXOFindByEstimatedInputSize(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	provider := given.Provider().GORM()

	utxo := &entity.UserUTXO{
		UserID:             112,
		OutputID:           602,
		BasketName:         "basket-E",
		Satoshis:           33000,
		EstimatedInputSize: 222,
		CreatedAt:          time.Now(),
		Status:             wdk.UTXOStatusUnproven,
	}
	require.NoError(t, provider.UserUTXOEntity().Create(t.Context(), utxo))

	// when:
	result, err := provider.UserUTXOEntity().
		Read().
		UserID(112).
		Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, uint64(222), result[0].EstimatedInputSize)
}

func TestUserUTXOUpdateSatoshisAndInputSize(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	provider := given.Provider().GORM()

	utxo := &entity.UserUTXO{
		UserID:             113,
		OutputID:           603,
		BasketName:         "basket-U",
		Satoshis:           12300,
		EstimatedInputSize: 144,
		CreatedAt:          time.Now(),
		Status:             wdk.UTXOStatusUnproven,
	}
	require.NoError(t, provider.UserUTXOEntity().Create(t.Context(), utxo))

	// when:
	updatedSatoshis := uint64(77777)
	updatedInputSize := uint64(180)
	err := provider.UserUTXOEntity().Update(t.Context(), &entity.UserUTXOUpdateSpecification{
		OutputID:           utxo.OutputID,
		Satoshis:           &updatedSatoshis,
		EstimatedInputSize: &updatedInputSize,
	})

	// then:
	require.NoError(t, err)

	// when:
	result, err := provider.UserUTXOEntity().
		Read().
		OutputID().Equals(utxo.OutputID).
		Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, updatedSatoshis, result[0].Satoshis)
	assert.Equal(t, updatedInputSize, result[0].EstimatedInputSize)
}

func TestUserUTXOFindByReservedByID(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	provider := given.Provider().GORM()

	reservedBy := uint(999)
	utxo := &entity.UserUTXO{
		UserID:             114,
		OutputID:           604,
		BasketName:         "basket-Y",
		Satoshis:           50100,
		EstimatedInputSize: 128,
		CreatedAt:          time.Now(),
		ReservedByID:       &reservedBy,
		Status:             wdk.UTXOStatusSending,
	}
	require.NoError(t, provider.UserUTXOEntity().Create(t.Context(), utxo))

	// when:
	result, err := provider.UserUTXOEntity().
		Read().
		UserID(114).
		Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.NotNil(t, result[0].ReservedByID)
	assert.Equal(t, reservedBy, *result[0].ReservedByID)
}
