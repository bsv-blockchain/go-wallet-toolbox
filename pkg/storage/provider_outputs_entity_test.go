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

func TestOutputCountFilters(t *testing.T) {
	tests := map[string]struct {
		filter func(reader crud.OutputReader)
		count  int64
	}{
		"all outputs": {
			count: 12,
		},
		"user only": {
			filter: func(r crud.OutputReader) { r.UserID().Equals(outputTestUser.ID) },
			count:  2,
		},
		"filter by spendable": {
			filter: func(r crud.OutputReader) { r.Spendable().Equals(true) },
			count:  9,
		},
		"filter by change": {
			filter: func(r crud.OutputReader) { r.Change().Equals(true) },
			count:  3,
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
		"filter by Satoshis (>= 1005)": {
			filter: func(r crud.OutputReader) { r.Satoshis().GreaterThanOrEqual(1005) },
			count:  5,
		},
		"filter by TransactionID = 1": {
			filter: func(r crud.OutputReader) { r.TransactionID().Equals(1) },
			count:  1,
		},
		"filter by Vout = 1": {
			filter: func(r crud.OutputReader) { r.Vout().Equals(1) },
			count:  2,
		},
		"filter by SpentBy": {
			filter: func(r crud.OutputReader) { r.SpentBy().Equals(3) },
			count:  1,
		},
		"tags contain alpha": {
			filter: func(r crud.OutputReader) {
				r.Tags().ContainAny("alpha")
			},
			count: 5,
		},
		"tags contain beta": {
			filter: func(r crud.OutputReader) {
				r.Tags().ContainAny("beta")
			},
			count: 5,
		},
		"tags contain gamma": {
			filter: func(r crud.OutputReader) {
				r.Tags().ContainAny("gamma")
			},
			count: 5,
		},
		"tags contain alpha OR beta (contain any)": {
			filter: func(r crud.OutputReader) {
				r.Tags().ContainAny("alpha", "beta")
			},
			count: 10,
		},
		"tags contain alpha OR gamma (contain any)": {
			filter: func(r crud.OutputReader) {
				r.Tags().ContainAny("alpha", "gamma")
			},
			count: 10,
		},
		"tags contain nothing (empty filter for any)": {
			filter: func(r crud.OutputReader) {
				r.Tags().ContainAny()
			},
			count: 12,
		},
		"tags must contain both alpha AND beta (contain all)": {
			filter: func(r crud.OutputReader) {
				r.Tags().ContainAll("alpha", "beta")
			},
			count: 0,
		},
		"tags must contain both beta AND gamma (contain all)": {
			filter: func(r crud.OutputReader) {
				r.Tags().ContainAll("beta", "gamma")
			},
			count: 5,
		},
		"tags contain nothing (empty filter for all)": {
			filter: func(r crud.OutputReader) {
				r.Tags().ContainAll()
			},
			count: 12,
		},
		"tags empty (outputs with no tags)": {
			filter: func(r crud.OutputReader) {
				r.Tags().Empty()
			},
			count: 2,
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
	outs, err := activeStorage.OutputsEntity().Read().ID(100).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, outs, 1)
	assert.Equal(t, uint(100), outs[0].ID)
	assert.Equal(t, outputTestUser.ID, outs[0].UserID)
	assert.Equal(t, int64(1), outs[0].Satoshis)
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

func TestOutputFindByID_WithTransactionJoin(t *testing.T) {
	activeStorage := seedDbWithOutputs(t)

	// when:
	outs, err := activeStorage.OutputsEntity().Read().ID(1).Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, outs, 1)
	require.NotNil(t, outs[0].TxID)

	assert.Equal(t, "alice_tx_1", *outs[0].TxID)
	assert.Equal(t, wdk.TxStatusUnprocessed, outs[0].TxStatus)
}

func TestOutputCountByTxID(t *testing.T) {
	// given:
	activeStorage := seedDbWithOutputs(t)

	tx := &entity.Transaction{
		ID:     999,
		TxID:   to.Ptr("txid-unique"),
		Status: wdk.TxStatusUnprocessed,
	}
	require.NoError(t, activeStorage.TransactionEntity().Create(t.Context(), tx))

	out := &entity.Output{
		UserID:        outputTestUser.ID,
		TransactionID: tx.ID,
		Vout:          0,
		Satoshis:      1234,
		Spendable:     true,
		Change:        false,
		Description:   "test output with specific txid",
		ProvidedBy:    "test-case",
		Purpose:       "unit test",
		Type:          "p2pkh",
		BasketName:    to.Ptr("special"),
	}
	require.NoError(t, activeStorage.OutputsEntity().Create(t.Context(), out))

	// when:
	count, err := activeStorage.OutputsEntity().Read().
		TxID().Equals("txid-unique").
		Count(t.Context())

	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestOutputCountByTxStatus(t *testing.T) {
	// given:
	activeStorage := seedDbWithOutputs(t)

	tx := &entity.Transaction{
		ID:     1000,
		TxID:   to.Ptr("txid-1"),
		Status: wdk.TxStatusUnproven,
	}
	require.NoError(t, activeStorage.TransactionEntity().Create(t.Context(), tx))

	out := &entity.Output{
		UserID:        outputTestUser.ID,
		TransactionID: tx.ID,
		Vout:          0,
		Satoshis:      1234,
		Spendable:     true,
		Change:        false,
		Description:   "test output with specific txid",
		ProvidedBy:    "test-case",
		Purpose:       "unit test",
		Type:          "p2pkh",
		BasketName:    to.Ptr("special"),
	}
	require.NoError(t, activeStorage.OutputsEntity().Create(t.Context(), out))

	// when:
	count, err := activeStorage.OutputsEntity().Read().
		TxStatus().Equals(wdk.TxStatusUnproven).
		Count(t.Context())

	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestOutputExposedGetter(t *testing.T) {
	// given:
	activeStorage := seedDbWithOutputs(t)

	tests := map[string]struct {
		filter      wdk.FindOutputsArgs
		expectedLen int
		then        func(outputs []wdk.TableOutput)
	}{
		"filter by OutputID": {
			filter:      wdk.FindOutputsArgs{OutputID: to.Ptr(uint(1))},
			expectedLen: 1,
			then: func(outputs []wdk.TableOutput) {
				assert.Equal(t, uint(1), outputs[0].OutputID)
			},
		},
		"filter by Satoshis": {
			filter:      wdk.FindOutputsArgs{Satoshis: to.Ptr(int64(1003))},
			expectedLen: 1,
			then: func(outputs []wdk.TableOutput) {
				assert.Equal(t, int64(1003), outputs[0].Satoshis)
			},
		},
		"filter by TransactionID": {
			filter:      wdk.FindOutputsArgs{TransactionID: to.Ptr(uint(1))},
			expectedLen: 1,
			then: func(outputs []wdk.TableOutput) {
				assert.Equal(t, uint(1), outputs[0].TransactionID)
			},
		},
		"filter by TxID": {
			filter:      wdk.FindOutputsArgs{TxID: to.Ptr("alice_tx_1")},
			expectedLen: 1,
			then: func(outputs []wdk.TableOutput) {
				assert.Equal(t, "alice_tx_1", *outputs[0].TxID)
			},
		},
		"filter by Vout": {
			filter:      wdk.FindOutputsArgs{Vout: to.Ptr(uint32(1))},
			expectedLen: 1,
			then: func(outputs []wdk.TableOutput) {
				assert.Equal(t, uint32(1), outputs[0].Vout)
			},
		},
		"filter by Change": {
			filter:      wdk.FindOutputsArgs{Change: to.Ptr(true)},
			expectedLen: 3,
			then: func(outputs []wdk.TableOutput) {
				for _, o := range outputs {
					assert.True(t, o.Change)
				}
			},
		},
		"filter by Spendable": {
			filter:      wdk.FindOutputsArgs{Spendable: to.Ptr(true)},
			expectedLen: 4,
			then: func(outputs []wdk.TableOutput) {
				for _, o := range outputs {
					assert.True(t, o.Spendable)
				}
			},
		},
		"filter by TxStatus": {
			filter:      wdk.FindOutputsArgs{TxStatus: []wdk.TxStatus{wdk.TxStatusUnprocessed}},
			expectedLen: 1,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			outputs, err := activeStorage.FindOutputsAuth(t.Context(), testusers.Alice.AuthID(), test.filter)

			// then:
			require.NoError(t, err)
			require.Len(t, outputs, test.expectedLen)

			// and:
			if test.then != nil {
				test.then(outputs)
			}
		})
	}
}

// seedDbWithOutputs inserts test outputs
func seedDbWithOutputs(t testing.TB) *storage.Provider {
	t.Helper()
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)

	activeStorage := given.Provider().GORM()

	for i := range 5 {
		txID := to.Ptr(fmt.Sprintf("alice_tx_%d", i+1))
		require.NoError(t, activeStorage.OutputsEntity().Create(t.Context(), &entity.Output{
			UserID:        testusers.Alice.ID,
			TransactionID: uint(i + 1),
			Vout:          uint32(i),
			Satoshis:      1000 + int64(i),
			Spendable:     i != 2,
			Change:        i%2 == 0,
			Description:   "output for alpha",
			ProvidedBy:    "test",
			Purpose:       "unit test",
			Type:          "p2pkh",
			BasketName:    to.Ptr("default"),
			Tags:          []string{"alpha"},
			TxID:          txID,
		}))
		require.NoError(t, activeStorage.TransactionEntity().Create(t.Context(), &entity.Transaction{
			TxID:      txID,
			Status:    to.IfThen(i == 0, wdk.TxStatusUnprocessed).ElseThen(wdk.TxStatusCompleted),
			UserID:    testusers.Alice.ID,
			Satoshis:  1000 + int64(i),
			Reference: fmt.Sprintf("reference for alice tx %d", i+1),
		}))
	}

	for i := range 5 {
		out := &entity.Output{
			UserID:        testusers.Bob.ID,
			TransactionID: uint(6 + i),
			Vout:          uint32(i),
			Satoshis:      1005 + int64(i),
			Spendable:     true,
			Change:        false,
			Description:   "output for beta+gamma",
			ProvidedBy:    "test",
			Purpose:       "unit test",
			Type:          "p2pkh",
			BasketName:    to.Ptr("default"),
			Tags:          []string{"beta", "gamma"},
		}
		require.NoError(t, activeStorage.OutputsEntity().Create(t.Context(), out))
	}

	for i := range 2 {
		tx := &entity.Output{
			ID:          uint(100 + i),
			UserID:      outputTestUser.ID,
			Satoshis:    1 + int64(i),
			Description: "test transaction from jerry",
			TxID:        to.Ptr(fmt.Sprintf("txid_jerry_%d", i)),
			SpentBy:     to.Ptr(uint(3 * i)),
		}
		require.NoError(t, activeStorage.OutputsEntity().Create(t.Context(), tx))
	}

	return activeStorage
}

var outputTestUser = struct{ ID int }{ID: 123}
