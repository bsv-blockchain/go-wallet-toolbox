package storage_test

import (
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/crud"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestKnownTxAttemptsFilters(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	provider := given.Provider()
	activeStorage := provider.GORM()

	tx1, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	provider.ARC().WhenQueryingTx(tx1.ID().String()).WillReturnTransactionWithoutMerklePath()
	provider.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})

	// when:
	for i := 0; i < 3; i++ {
		_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
		require.NoError(t, err)
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

func TestKnownTxBlockHeightFilters(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	const blockHeight = uint32(12345)

	provider := given.Provider()
	activeStorage := provider.GORM()

	tx1, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	provider.ARC().WhenQueryingTx(tx1.ID().String()).WillReturnTransactionWithBlockHeight(12345)
	provider.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})

	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(tx1.ID().String()).
		WithBlockHeight(to.Ptr(blockHeight)).
		IsMined()

	tests := map[string]struct {
		filter func(r crud.KnownTxReader)
		expect int64
	}{
		"equals 12345": {
			filter: func(r crud.KnownTxReader) { r.BlockHeight().Equals(12345) },
			expect: 1,
		},
		"greater than 10000": {
			filter: func(r crud.KnownTxReader) { r.BlockHeight().GreaterThan(10000) },
			expect: 1,
		},
		"between 12340 and 12350": {
			filter: func(r crud.KnownTxReader) { r.BlockHeight().Between(12340, 12350) },
			expect: 1,
		},
		"less than 20000": {
			filter: func(r crud.KnownTxReader) { r.BlockHeight().LessThan(20000) },
			expect: 1,
		},
		"in list": {
			filter: func(r crud.KnownTxReader) { r.BlockHeight().In([]uint32{100, 12345}...) },
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

func TestKnownTxBlockHashFilters(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	provider := given.Provider()
	activeStorage := provider.GORM()

	tx1, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	mp := testutils.MockValidMerklePath(t, tx1.ID().String(), 2000)
	provider.ARC().WhenQueryingTx(tx1.ID().String()).
		WillReturnTransactionWithMerklePath(mp)
	provider.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})

	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	expectedBlockHash := testservices.TestBlockHash

	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(tx1.ID().String()).
		WithBlockHash(to.Ptr(expectedBlockHash)).
		IsMined()

	tests := map[string]struct {
		filter func(r crud.KnownTxReader)
		expect int64
	}{
		"equals block hash": {
			filter: func(r crud.KnownTxReader) { r.BlockHash().Equals(expectedBlockHash) },
			expect: 1,
		},
		"in list": {
			filter: func(r crud.KnownTxReader) {
				r.BlockHash().In([]string{"0000000000000000000000000000000000000000000000000000000000000000", expectedBlockHash}...)
			},
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

func TestKnownTxMerkleRootFilters(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	provider := given.Provider()
	activeStorage := provider.GORM()

	tx1, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	mp := testutils.MockValidMerklePath(t, tx1.ID().String(), 2000)

	provider.ARC().WhenQueryingTx(tx1.ID().String()).
		WillReturnTransactionWithMerklePath(mp)
	provider.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})

	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	txidHash, err := chainhash.NewHashFromHex(tx1.ID().String())
	require.NoError(t, err)

	expectedRoot, err := mp.ComputeRootHex(to.Ptr(txidHash.String()))
	require.NoError(t, err)

	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(tx1.ID().String()).
		WithMerkleRoot(to.Ptr(expectedRoot)).
		IsMined()

	tests := map[string]struct {
		filter func(r crud.KnownTxReader)
		expect int64
	}{
		"equals merkle root": {
			filter: func(r crud.KnownTxReader) { r.MerkleRoot().Equals(expectedRoot) },
			expect: 1,
		},
		"in list": {
			filter: func(r crud.KnownTxReader) { r.MerkleRoot().In([]string{"deadbeef", expectedRoot}...) },
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

func TestKnownTxStatusFilters(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	provider := given.Provider()
	activeStorage := provider.GORM()

	txUnmined, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(50_000)
	provider.ARC().WhenQueryingTx(txUnmined.ID().String()).
		WillReturnTransactionWithoutMerklePath()

	txMined, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(75_000)
	mp := testutils.MockValidMerklePath(t, txMined.ID().String(), 2000)
	provider.ARC().WhenQueryingTx(txMined.ID().String()).
		WillReturnTransactionWithMerklePath(mp)

	provider.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})

	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txUnmined.ID().String()).
		WithStatus(wdk.ProvenTxStatusUnmined)
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txMined.ID().String()).
		WithStatus(wdk.ProvenTxStatusCompleted)

	tests := map[string]struct {
		filter func(r crud.KnownTxReader)
		expect int64
	}{
		"status == unmined": {
			filter: func(r crud.KnownTxReader) { r.Status().Equals(wdk.ProvenTxStatusUnmined) },
			expect: 1,
		},
		"status == completed": {
			filter: func(r crud.KnownTxReader) { r.Status().Equals(wdk.ProvenTxStatusCompleted) },
			expect: 1,
		},
		"status in (completed, unmined)": {
			filter: func(r crud.KnownTxReader) {
				r.Status().In(wdk.ProvenTxStatusCompleted, wdk.ProvenTxStatusUnmined)
			},
			expect: 2,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// when:
			reader := activeStorage.KnownTxEntity().Read()
			tc.filter(reader)

			// then:
			count, err := reader.Count(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tc.expect, count)
		})
	}
}

func TestKnownTxNotifiedFilters(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	provider := given.Provider()
	activeStorage := provider.GORM()

	tx1, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	provider.ARC().WhenQueryingTx(tx1.ID().String()).
		WillReturnTransactionWithoutMerklePath()
	provider.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})

	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(tx1.ID().String()).
		IsNotified(false)

	t.Run("equals false", func(t *testing.T) {
		// when:
		reader := activeStorage.KnownTxEntity().Read()
		reader.Notified().Equals(false)

		// then:
		count, countErr := reader.Count(t.Context())
		require.NoError(t, countErr)
		assert.Equal(t, int64(1), count)
	})

	t.Run("not equal true (i.e., false)", func(t *testing.T) {
		// when:
		reader := activeStorage.KnownTxEntity().Read()
		reader.Notified().NotEquals(true)

		// then:
		count, countErr := reader.Count(t.Context())
		require.NoError(t, countErr)
		assert.Equal(t, int64(1), count)
	})

	// when:
	mp := testutils.MockValidMerklePath(t, tx1.ID().String(), 2000)
	provider.ARC().WhenQueryingTx(tx1.ID().String()).
		WillReturnTransactionWithMerklePath(mp)
	_, err = activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	// then:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(tx1.ID().String()).
		IsNotified(true).
		IsMined()

	t.Run("equals true", func(t *testing.T) {
		// when:
		reader := activeStorage.KnownTxEntity().Read()
		reader.Notified().Equals(true)

		// then:
		count, err := reader.Count(t.Context())
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("not equal false (i.e., true)", func(t *testing.T) {
		// when:
		reader := activeStorage.KnownTxEntity().Read()
		reader.Notified().NotEquals(false)

		// then:
		count, err := reader.Count(t.Context())
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}
