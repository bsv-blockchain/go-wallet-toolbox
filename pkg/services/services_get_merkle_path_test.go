package services_test

import (
	"context"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMerklePath(t *testing.T) {
	tx := txtestabilities.GivenTX().WithInput(100).WithP2PKHOutput(99).TX()
	txID := tx.TxID().String()

	someSecondHash, errHash := chainhash.NewHashFromHex("27a53423aa3e5d5c46bf30be53a9998dd247daf758847f244f82d430be71de6e")
	require.NoError(t, errHash)

	t.Run("return error when all services are unreachable", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		response, err := services.MerklePath(context.Background(), txID)

		// then:
		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("return result without Merkle Path when transaction is not mined yet", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		given.ARC().IsUpAndRunning()

		// and:
		given.ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		response, err := services.MerklePath(context.Background(), txID)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, wdk.MerklePathResult{
			Name:                   "ARC",
			MerklePath:             nil,
			TransactionBlockHeader: nil,
		}, *response)
	})

	t.Run("get merkle path from arc", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		given.ARC().IsUpAndRunning()

		merklePath := sdk.MerklePath{
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

		merkleRoot, err := merklePath.ComputeRootHex(nil)
		require.NoError(t, err, "failed to compute block hash from merkle path, wrong test setup")

		// and:
		given.ARC().WhenQueryingTx(txID).WillReturnTransactionWithMerklePath(merklePath)

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		response, err := services.MerklePath(context.Background(), txID)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, wdk.MerklePathResult{
			Name:       "ARC",
			MerklePath: &merklePath,
			TransactionBlockHeader: &wdk.TransactionBlockHeader{
				Height:     2000,
				Hash:       testservices.TestBlockHash,
				MerkleRoot: merkleRoot,
			},
		}, *response)
	})

}
