package whatsonchain_test

import (
	"fmt"
	"net/http"
	"testing"

	tst "github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func TestMerklePath_Success(t *testing.T) {
	// Given
	given := tst.Given(t)
	svc := given.NewWoCService()

	txID := tst.TestTxID
	siblingHash := tst.TestSiblingHash
	txIDHash := tst.MustHashFromHex(txID)
	siblingHashObj := tst.MustHashFromHex(siblingHash)

	merklePath := transaction.MerklePath{
		BlockHeight: tst.BlockHeight,
		Path: [][]*transaction.PathElement{
			{
				{
					Offset: 0,
					Hash:   txIDHash,
					Txid:   to.Ptr(true),
				},
				{
					Offset: 1,
					Hash:   siblingHashObj,
				},
			},
		},
	}

	merkleRoot, err := merklePath.ComputeRootHex(nil)
	require.NoError(t, err, "failed to compute merkle root")

	mockMerklePathResponse := fmt.Sprintf(`{
		"index": 0,
		"txOrId": "%s",
		"target": "%s",
		"nodes": ["%s"]
	}`, txID, tst.TestTargetHash, siblingHash)

	given.WhatsOnChain().WillRespondWithMerklePath(http.StatusOK, txID, mockMerklePathResponse)

	mockBlockHeaderResponse := fmt.Sprintf(`{
		"hash": "%s",
		"height": %d,
		"merkleRoot": "%s"
	}`, tst.TestTargetHash, tst.BlockHeight, merkleRoot)

	given.WhatsOnChain().WillRespondWithBlockHeader(http.StatusOK, tst.TestTargetHash, mockBlockHeaderResponse)

	expected := &wdk.MerklePathResult{
		Name:       "WhatsOnChain",
		MerklePath: &merklePath,
		Header: &wdk.BlockHeader{
			Height:     tst.BlockHeight,
			MerkleRoot: merkleRoot,
			Hash:       tst.TestTargetHash,
		},
	}

	// When
	res, err := svc.MerklePath(t.Context(), txID)

	// Then
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, expected, res)
}
