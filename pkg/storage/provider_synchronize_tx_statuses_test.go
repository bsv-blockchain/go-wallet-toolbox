package storage_test

import (
	"context"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/testutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

func TestSynchronizeTx(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and:
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	givenProvider.ARC().WhenQueryingTx(txSpec.ID()).WillReturnWithMindedTx()

	// when:
	err := activeStorage.SynchronizeTransactionStatuses(context.Background())

	// then:
	require.NoError(t, err)

	// and:
	listOutputsResult, err := activeStorage.ListOutputs(context.Background(), testusers.Alice.AuthID(), wdk.ListOutputsArgs{Limit: 10, IncludeTransactions: true})
	require.NoError(t, err)
	require.NotNil(t, listOutputsResult.BEEF)
	beef := testutils.BEEFFromHex(t, *listOutputsResult.BEEF)
	require.Len(t, beef.Transactions, 1) // should be one transaction, because we just made it "mined" so parent transaction should not be included
	require.NotNil(t, beef.FindTransaction(txSpec.ID()))
}

func TestFailedSyncExceedsMaxAttempts(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and:
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	givenProvider.ARC().WhenQueryingTx(txSpec.ID()).WillReturnTransactionWithoutMerklePath()

	// when:
	for range defs.DefaultSynchronizeTxStatuses().MaxAttempts + 1 {
		err := activeStorage.SynchronizeTransactionStatuses(context.Background())
		require.NoError(t, err)
	}

	// then:
	listOutputsResult, err := activeStorage.ListOutputs(context.Background(), testusers.Alice.AuthID(), wdk.ListOutputsArgs{Limit: 10, IncludeTransactions: true})
	require.NoError(t, err)
	require.NotNil(t, listOutputsResult.BEEF)
	beef := testutils.BEEFFromHex(t, *listOutputsResult.BEEF)
	require.Len(t, beef.Transactions, 0) // should be no transactions, because the proven tx has been set to "failed"
}

func TestSynchronizeTxEdgeCases(t *testing.T) {
	tests := map[string]struct {
		setupARCMock func(arcQueryFixture testservices.ARCQueryFixture)
	}{
		"ARC returns transaction without MerklePath": {
			setupARCMock: func(arcQueryFixture testservices.ARCQueryFixture) {
				arcQueryFixture.WillReturnTransactionWithoutMerklePath()
			},
		},
		"ARC returns no body": {
			setupARCMock: func(arcQueryFixture testservices.ARCQueryFixture) {
				arcQueryFixture.WillReturnNoBody()
			},
		},
		"ARC is unreachable": {
			setupARCMock: func(arcQueryFixture testservices.ARCQueryFixture) {
				arcQueryFixture.WillBeUnreachable()
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			// given:
			givenProvider := given.Provider()
			activeStorage := givenProvider.GORM()

			// and:
			txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

			// and:
			arcQueryFixture := givenProvider.ARC().WhenQueryingTx(txSpec.ID())
			test.setupARCMock(arcQueryFixture)

			// when:
			err := activeStorage.SynchronizeTransactionStatuses(context.Background())

			// then:
			require.NoError(t, err)

			// NOTE: Error is not returned, because this action tries to synchronize all transactions and skips those that are not found or have no Merkle Path.
		})
	}
}
