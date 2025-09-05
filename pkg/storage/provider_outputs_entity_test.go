package storage_test

import (
	"testing"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/crud"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputCountFilters(t *testing.T) {
	tests := map[string]struct {
		filter func(reader crud.OutputReader)
		count  int64
	}{
		"all outputs": {
			count: 10,
		},
		"user only": {
			filter: func(r crud.OutputReader) { r.UserID().Equals(outputTestUser.ID) },
			count:  10,
		},
		"filter by spendable": {
			filter: func(r crud.OutputReader) { r.Spendable().Equals(true) },
			count:  9,
		},
		"filter by change": {
			filter: func(r crud.OutputReader) { r.Change().Equals(true) },
			count:  5,
		},
		"filter by basket name": {
			filter: func(r crud.OutputReader) { r.BasketName().Equals("default") },
			count:  10,
		},
		"since now (no results)": {
			filter: func(r crud.OutputReader) {
				since := time.Now().Add(time.Minute)
				r.Since(since, entity.SinceFieldCreatedAt)
			},
			count: 0,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			activeStorage := seedDbWithOutputs(t)
			reader := activeStorage.OutputsEntity().Read()

			// when:
			if test.filter != nil {
				test.filter(reader)
			}
			count, err := reader.Count(t.Context())

			// then:
			require.NoError(t, err)
			assert.Equal(t, test.count, count)
		})
	}
}

func TestOutputUpdate_AllFields(t *testing.T) {
	// given:
	activeStorage := seedDbWithOutputs(t)

	newSpendable := to.Ptr(false)
	newDescription := "updated description"
	newScript := []byte{0x76, 0xa9, 0x14}
	newCustom := "custom instructions"

	// when:
	err := activeStorage.OutputsEntity().Update(t.Context(), &entity.OutputUpdateSpecification{
		ID:            1,
		Spendable:     newSpendable,
		Description:   &newDescription,
		LockingScript: &newScript,
		CustomInstr:   &newCustom,
	})
	require.NoError(t, err)

	// then:
	outputs, err := activeStorage.OutputsEntity().Read().ID(1).Find(t.Context())
	require.NoError(t, err)
	require.Len(t, outputs, 1)

	assert.Equal(t, *newSpendable, outputs[0].Spendable)

	assert.Equal(t, newDescription, outputs[0].Description)

	assert.Equal(t, newScript, outputs[0].LockingScript)

	require.NotNil(t, outputs[0].CustomInstructions)
	assert.Equal(t, newCustom, *outputs[0].CustomInstructions)
}

func TestOutputFindByID(t *testing.T) {
	// given:
	activeStorage := seedDbWithOutputs(t)

	// when:
	outs, err := activeStorage.OutputsEntity().Read().ID(1).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, outs, 1)
	assert.Equal(t, uint(1), outs[0].ID)
	assert.Equal(t, outputTestUser.ID, outs[0].UserID)
	assert.Equal(t, int64(1000), outs[0].Satoshis)
}

func TestOutputPagedFind(t *testing.T) {
	// given:
	activeStorage := seedDbWithOutputs(t)

	// when:
	paged, err := activeStorage.OutputsEntity().Read().Paged(5, 5, false).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, paged, 5)
	assert.Equal(t, uint(6), paged[0].ID)
	assert.Equal(t, uint(10), paged[4].ID)
}

// seedDbWithOutputs inserts test outputs
func seedDbWithOutputs(t testing.TB) *storage.Provider {
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)

	activeStorage := given.Provider().GORM()

	for i := range 10 {
		out := &entity.Output{
			UserID:        outputTestUser.ID,
			TransactionID: 1,
			Vout:          uint32(i),
			Satoshis:      1000 + int64(i),
			Spendable:     i != 2,
			Change:        i%2 == 0,
			Description:   "test output",
			ProvidedBy:    "test",
			Purpose:       "unit test",
			Type:          "p2pkh",
			BasketName:    to.Ptr("default"),
			SpentBy:       nil,
		}
		require.NoError(t, activeStorage.OutputsEntity().Create(t.Context(), out))
	}

	return activeStorage
}

var outputTestUser = struct{ ID int }{ID: 123}
