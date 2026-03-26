package services_test

import (
	"fmt"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arc"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
	btst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	tst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
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
		services := given.Services().New()

		// when:
		response, err := services.MerklePath(t.Context(), txID)

		// then:
		require.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("return result without Merkle Path when transaction is not mined yet", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		given.ARC().IsUpAndRunning()

		// and:
		given.ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()

		// and:
		services := given.Services().New()

		// when:
		response, err := services.MerklePath(t.Context(), txID)

		// then:
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, arc.ServiceName, response.Name)
		assert.Nil(t, response.MerklePath)
		assert.Nil(t, response.BlockHeader)
		assert.Len(t, response.Notes, 1)
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
		services := given.Services().New()

		// when:
		response, err := services.MerklePath(t.Context(), txID)

		// then:
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, arc.ServiceName, response.Name)
		assert.Equal(t, merklePath, *response.MerklePath)
		assert.Equal(t, wdk.MerklePathBlockHeader{
			Height:     2000,
			Hash:       testservices.TestBlockHash,
			MerkleRoot: merkleRoot,
		}, *response.BlockHeader)
		assert.Len(t, response.Notes, 1)
	})

	t.Run("get merkle path from WoC", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		txHash := tst.MustHashFromHex(tst.TestTxID)
		siblingHash := tst.MustHashFromHex(tst.TestSiblingHash)

		merklePath := sdk.MerklePath{
			BlockHeight: tst.TestBlockHeight,
			Path: [][]*sdk.PathElement{
				{
					{
						Offset: 0,
						Hash:   txHash,
						Txid:   to.Ptr(true),
					},
					{
						Offset: 1,
						Hash:   siblingHash,
					},
				},
			},
		}

		merkleRoot, err := merklePath.ComputeRootHex(nil)
		require.NoError(t, err, "failed to compute merkle root")

		given.WhatsOnChain().WhenQueryingMerklePath(tst.TestTxID).WillReturnTSCProof(200, `[{
			"index": 0,
			"txOrId": "`+tst.TestTxID+`",
			"target": "`+tst.TestTargetHash+`",
			"nodes": ["`+tst.TestSiblingHash+`"]
		}]`)

		blockHeaderJSON := fmt.Sprintf(`{
			"hash": "%s",
			"height": %d,
			"merkleRoot": "%s"
		}`, tst.TestTargetHash, tst.TestBlockHeight, merkleRoot)

		given.WhatsOnChain().WhenQueryingBlockHeader(tst.TestTargetHash).WillReturnBlockHeaderJSON(200, blockHeaderJSON)
		services := given.Services().New()

		// when:
		response, err := services.MerklePath(t.Context(), tst.TestTxID)

		// then:
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, whatsonchain.ServiceName, response.Name)
		assert.Equal(t, merklePath, *response.MerklePath)
		assert.Equal(t, wdk.MerklePathBlockHeader{
			Height:     tst.TestBlockHeight,
			Hash:       tst.TestTargetHash,
			MerkleRoot: merkleRoot,
		}, *response.BlockHeader)
		assert.Len(t, response.Notes, 1)
	})

	t.Run("get merkle path from Bitails", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		txID := btst.TestTxID
		blockHash := btst.TestTargetHash
		sibling := btst.TestSiblingHash

		txHash := btst.HashFromHex(t, txID)
		siblingHash := btst.HashFromHex(t, sibling)

		merklePath := sdk.MerklePath{
			BlockHeight: btst.TestBlockHeight,
			Path: [][]*sdk.PathElement{{
				{
					Offset: 0,
					Hash:   txHash,
					Txid:   to.Ptr(true),
				},
				{
					Offset: 1,
					Hash:   siblingHash,
				},
			}},
		}

		merkleRoot, err := merklePath.ComputeRootHex(&txID)
		require.NoError(t, err, "failed to compute merkle root")

		given.Bitails().WillReturnTscProof(txID, blockHash, 0, []string{sibling})

		headerWithCorrectMerkleRoot := btst.FakeHeaderHexWithMerkleRoot(t, merkleRoot)
		given.Bitails().WillReturnBlockHeader(blockHash, headerWithCorrectMerkleRoot)

		given.Bitails().WillReturnBranchProof(txID, blockHash, merkleRoot, []map[string]string{
			{
				"pos":  "0",
				"hash": sibling,
			},
		})
		given.Bitails().WillReturnTxStatus(txID, btst.TestBlockHeight)

		services := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		response, err := services.MerklePath(t.Context(), txID)

		// then:
		require.NoError(t, err)
		require.NotNil(t, response)

		require.Equal(t, bitails.ServiceName, response.Name)
		require.Equal(t, merklePath, *response.MerklePath)
		require.Equal(t, &wdk.MerklePathBlockHeader{
			Height:     btst.TestBlockHeight,
			Hash:       blockHash,
			MerkleRoot: merkleRoot,
		}, response.BlockHeader)
		require.NotEmpty(t, response.Notes)
		require.Equal(t, "getMerklePathSuccess", response.Notes[0].What)
	})
}
