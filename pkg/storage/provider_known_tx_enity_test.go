package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/crud"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKnownTxAttemptsFilters(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	provider := given.Provider()
	activeStorage := provider.GORM()

	tx1, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	provider.ARC().WhenQueryingTx(tx1.ID().String()).WillReturnTransactionWithoutMerklePath()

	// when:
	for i := 0; i < 3; i++ {
		require.NoError(t, activeStorage.SynchronizeTransactionStatuses(t.Context()))
	}

	// then:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(tx1.ID().String()).
		WithAttempts(3).
		NotMined()

	tests := map[string]struct {
		filter func(r crud.KnownTxReader)
		expect int64
	}{
		"equals 3": {
			filter: func(r crud.KnownTxReader) { r.Attempts().Equals(3) },
			expect: 1,
		},
		"greater than 2": {
			filter: func(r crud.KnownTxReader) { r.Attempts().GreaterThan(2) },
			expect: 1,
		},
		"between 2 and 3": {
			filter: func(r crud.KnownTxReader) { r.Attempts().Between(2, 3) },
			expect: 1,
		},
		"less than 4": {
			filter: func(r crud.KnownTxReader) { r.Attempts().LessThan(4) },
			expect: 1,
		},
		"in list": {
			filter: func(r crud.KnownTxReader) { r.Attempts().In([]uint64{1, 3}...) },
			expect: 1,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when:
			reader := activeStorage.KnownTxEntity().Read()
			if test.filter != nil {
				test.filter(reader)
			}

			// then:
			count, err := reader.Count(t.Context())
			require.NoError(t, err)
			assert.Equal(t, test.expect, count)
		})
	}
}
