package whatsonchain_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	tst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func TestMerklePath_Success(t *testing.T) {
	// given:
	given := tst.Given(t)
	svc := given.NewWoCService()

	txID := tst.TestTxID
	siblingHash := tst.TestSiblingHash
	txIDHash := tst.MustHashFromHex(txID)
	siblingHashObj := tst.MustHashFromHex(siblingHash)

	merklePath := transaction.MerklePath{
		BlockHeight: tst.TestBlockHeight,
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

	mockMerklePathResponse := fmt.Sprintf(`[{
		"index": 0,
		"txOrId": "%s",
		"target": "%s",
		"nodes": ["%s"]
	}]`, txID, tst.TestTargetHash, siblingHash)

	given.WhatsOnChain().WillRespondWithMerklePath(http.StatusOK, txID, mockMerklePathResponse)

	mockBlockHeaderResponse := fmt.Sprintf(`{
		"hash": "%s",
		"height": %d,
		"merkleRoot": "%s"
	}`, tst.TestTargetHash, tst.TestBlockHeight, merkleRoot)

	given.WhatsOnChain().WillRespondWithBlockHeader(http.StatusOK, tst.TestTargetHash, mockBlockHeaderResponse)

	expected := &wdk.MerklePathResult{
		Name:       "WhatsOnChain",
		MerklePath: &merklePath,
		BlockHeader: &wdk.MerklePathBlockHeader{
			Height:     tst.TestBlockHeight,
			MerkleRoot: merkleRoot,
			Hash:       tst.TestTargetHash,
		},
	}

	// when:
	res, err := svc.MerklePath(t.Context(), txID)

	// then:
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, expected, res)
}
