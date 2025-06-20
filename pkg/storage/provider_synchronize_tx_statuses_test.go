package storage_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/testutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

const wocEndpointRegex = "whatsonchain(.*)headers"

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
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()

	// when:
	err := activeStorage.SynchronizeTransactionStatuses(context.Background())

	// then:
	require.NoError(t, err)

	// and:
	require.Equal(t, 1, givenProvider.ServicesSniffer().CountCallsByRegex(wocEndpointRegex))

	// and:
	listOutputsResult, err := activeStorage.ListOutputs(context.Background(), testusers.Alice.AuthID(), wdk.ListOutputsArgs{Limit: 10, IncludeTransactions: true})
	require.NoError(t, err)
	require.NotNil(t, listOutputsResult.BEEF)
	beef := testutils.BEEFFromHex(t, *listOutputsResult.BEEF)
	require.Len(t, beef.Transactions, 1) // should be one transaction, because we just made it "mined" so parent transaction should not be included
	require.NotNil(t, beef.FindTransaction(txSpec.ID()))
}

func TestSynchronizeTxEvenIfChainTipIsUnreachable(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and:
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	givenProvider.ARC().WhenQueryingTx(txSpec.ID()).WillReturnWithMindedTx()

	// NOTE: WhatsOnChain is unreachable, so we simulate that the chain tip is not available
	_ = givenProvider.WhatsOnChain().WillBeUnreachable()

	// when:
	err := activeStorage.SynchronizeTransactionStatuses(context.Background())

	// then:
	require.NoError(t, err)
}

func TestSynchronizeTxForTheSameBlockHeightTwice(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and:
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	servicesSniffer := givenProvider.ServicesSniffer()
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()

	// when:
	err := activeStorage.SynchronizeTransactionStatuses(context.Background())

	// then:
	require.NoError(t, err)

	// and:
	require.Equal(t, 1, servicesSniffer.CountCallsByRegex(fmt.Sprintf("arc(.*)tx\\/%s", txSpec.ID())))

	// when:
	err = activeStorage.SynchronizeTransactionStatuses(context.Background())

	// then:
	require.NoError(t, err)

	// and:
	require.Equal(t, 2, servicesSniffer.CountCallsByRegex(wocEndpointRegex))
	require.Equal(t, 1, servicesSniffer.CountCallsByRegex(fmt.Sprintf("arc(.*)tx\\/%s", txSpec.ID())))
	// NOTE: The second call should not trigger a request for the transaction, because the block height is the same
}

func TestSynchronizeTxForTwoDifferentBlockHeights(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// and:
	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	servicesSniffer := givenProvider.ServicesSniffer()
	givenProvider.ARC().WhenQueryingTx(txSpec.ID()).WillReturnWithMindedTx()
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()

	// when:
	err := activeStorage.SynchronizeTransactionStatuses(context.Background())

	// then:
	require.NoError(t, err)

	// and:
	require.Equal(t, 1, servicesSniffer.CountCallsByRegex(wocEndpointRegex))

	// given:
	givenProvider.WhatsOnChain().
		OnTipBlockHeaderWillRespondWithOneElementList(
			testservices.WithTipBlockHeaderHeight(testservices.TestBlockHeight + 1),
		)

	// when:
	err = activeStorage.SynchronizeTransactionStatuses(context.Background())

	// then:
	require.NoError(t, err)

	// and:
	// NOTE: The second call should also trigger a request to WhatsOnChain, because the block height is different
	require.Equal(t, 2, servicesSniffer.CountCallsByRegex(wocEndpointRegex))
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
			givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()
			test.setupARCMock(arcQueryFixture)

			// when:
			err := activeStorage.SynchronizeTransactionStatuses(context.Background())

			// then:
			require.NoError(t, err)

			// NOTE: Error is not returned, because this action tries to synchronize all transactions and skips those that are not found or have no Merkle Path.
		})
	}
}
