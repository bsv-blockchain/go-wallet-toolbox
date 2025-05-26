package storage_test

import (
	"context"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/testutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
	"testing"
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
	givenProvider.ARC().WhenQueryingTx(txSpec.ID()).WillReturnTransactionWithMerklePath(mockValidMerklePath(t, txSpec.TX()))

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

func mockValidMerklePath(t testing.TB, tx *sdk.Transaction) sdk.MerklePath {
	t.Helper()

	someSecondHash, errHash := chainhash.NewHashFromHex("27a53423aa3e5d5c46bf30be53a9998dd247daf758847f244f82d430be71de6e")
	require.NoError(t, errHash)

	return sdk.MerklePath{
		BlockHeight: 2000,
		Path: [][]*sdk.PathElement{
			{
				{
					Offset: 0,
					Hash:   tx.TxID(),
					Txid:   to.Ptr(true),
				},
				{
					Offset: 1,
					Hash:   someSecondHash,
				},
			},
		},
	}
}
