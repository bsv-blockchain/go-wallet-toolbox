package whatsonchain

import (
	"context"
	"fmt"
	"net/http"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/to"
)

// tscProof represents the response from the WoC /tx/{txid}/proof/tsc endpoint
type tscProof struct {
	Index  int      `json:"index"`
	Nodes  []string `json:"nodes"`
	Target string   `json:"target"` // block hash
	TxOrID string   `json:"txOrId"` // txid
}

// blockHeaderResponse represents the response from WoC /block/{hash}/header
type blockHeaderResponse struct {
	Height     int    `json:"height"`
	MerkleRoot string `json:"merkleRoot"`
}

// MerklePath retrieves the merkle path for a transaction using WoC TSC proof.
func (woc *WhatsOnChain) MerklePath(ctx context.Context, txID string) (*wdk.MerklePathResult, error) {
	proof, err := woc.getTscProof(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get TSC proof: %w", err)
	}
	if proof == nil {
		// Proof not found
		return &wdk.MerklePathResult{Name: ServiceName}, nil
	}

	header, err := woc.hashToHeader(ctx, proof.Target)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block header: %w", err)
	}

	merklePath, err := convertTscProofToMerklePath(txID, proof.Index, proof.Nodes, header.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to convert proof to merkle path: %w", err)
	}

	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	if err != nil {
		return nil, fmt.Errorf("failed to compute merkle root: %w", err)
	}
	if merkleRoot != header.MerkleRoot {
		return nil, fmt.Errorf("computed merkle root %q does not match block header %q", merkleRoot, header.MerkleRoot)
	}

	return &wdk.MerklePathResult{
		Name:       ServiceName,
		MerklePath: merklePath,
		Header:     header,
	}, nil
}

func (woc *WhatsOnChain) hashToHeader(ctx context.Context, blockHash string) (*wdk.BlockHeader, error) {
	var hdrResp blockHeaderResponse
	res, err := woc.httpClient.R().
		SetContext(ctx).
		SetResult(&hdrResp).
		AddRetryCondition(retryOnTooManyRequestsStatus).
		Get(fmt.Sprintf("%s/block/%s/header", woc.url, blockHash))

	if err != nil {
		return nil, fmt.Errorf("failed to fetch block header: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching block header", res.StatusCode())
	}

	height, err := to.UInt32(hdrResp.Height)
	if err != nil {
		return nil, fmt.Errorf("invalid block height %d: %w", hdrResp.Height, err)
	}

	return &wdk.BlockHeader{
		Height:     height,
		Hash:       blockHash,
		MerkleRoot: hdrResp.MerkleRoot,
	}, nil
}

// getTscProof retrieves the TSC proof from WoC.
func (woc *WhatsOnChain) getTscProof(ctx context.Context, txID string) (*tscProof, error) {
	var proof tscProof
	req := woc.httpClient.R().
		SetContext(ctx).
		SetResult(&proof).
		AddRetryCondition(retryOnTooManyRequestsStatus)

	res, err := req.Get(fmt.Sprintf("%s/tx/%s/proof/tsc", woc.url, txID))
	if err != nil {
		return nil, fmt.Errorf("failed to query TSC proof: %w", err)
	}
	if res.StatusCode() == http.StatusNotFound {
		return nil, nil // No proof found
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching TSC proof", res.StatusCode())
	}

	return &proof, nil
}

func convertTscProofToMerklePath(txid string, index int, nodes []string, blockHeight uint32) (*transaction.MerklePath, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes provided in TSC proof for txid %s", txid)
	}

	txidHash, err := chainhash.NewHashFromHex(txid)
	if err != nil {
		return nil, fmt.Errorf("invalid txid: %w", err)
	}

	level0, nextIndex, err := buildLevel0PathElement(txid, txidHash, nodes[0], index)
	if err != nil {
		return nil, fmt.Errorf("failed to build level 0 path element: %w", err)
	}

	upperLevels, err := buildUpperLevels(nodes, 1, nextIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to build upper levels: %w", err)
	}

	treeHeight := len(nodes)
	path := make([][]*transaction.PathElement, treeHeight)
	path[0] = level0
	for i := 1; i < treeHeight; i++ {
		path[i] = upperLevels[i]
	}

	return transaction.NewMerklePath(blockHeight, path), nil
}

func buildLevel0PathElement(txid string, txidHash *chainhash.Hash, node string, index int) ([]*transaction.PathElement, int, error) {
	isOdd := index%2 == 1
	siblingIndex := index ^ 1

	sibling, err := createPathElement(node, siblingIndex, true, txid)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid node hash at level 0: %w", err)
	}

	txidLeaf := &transaction.PathElement{
		Offset: must.ConvertToUInt64(index),
		Hash:   txidHash,
		Txid:   to.Ptr(true),
	}

	var level0 []*transaction.PathElement
	if isOdd {
		level0 = []*transaction.PathElement{sibling, txidLeaf}
	} else {
		level0 = []*transaction.PathElement{txidLeaf, sibling}
	}

	nextIndex := index >> 1
	return level0, nextIndex, nil
}

func buildUpperLevels(nodes []string, startLevel int, startIndex int) ([][]*transaction.PathElement, error) {
	treeHeight := len(nodes)
	path := make([][]*transaction.PathElement, treeHeight)

	currentIndex := startIndex

	for level := startLevel; level < treeHeight; level++ {
		siblingIndex := currentIndex ^ 1

		sibling, err := createPathElement(nodes[level], siblingIndex, false, "")
		if err != nil {
			return nil, fmt.Errorf("invalid node hash at level %d: %w", level, err)
		}

		path[level] = []*transaction.PathElement{sibling}
		currentIndex >>= 1
	}

	return path, nil
}

// createPathElement builds a PathElement given node string and sibling index.
func createPathElement(node string, siblingIndex int, isLevel0 bool, txid string) (*transaction.PathElement, error) {
	const duplicateNodeMarker = "*"

	element := &transaction.PathElement{
		Offset: must.ConvertToUInt64(siblingIndex),
	}

	if node == duplicateNodeMarker || (isLevel0 && node == txid) {
		element.Duplicate = to.Ptr(true)
	} else {
		nodeHash, err := chainhash.NewHashFromHex(node)
		if err != nil {
			return nil, fmt.Errorf("invalid node hash %q: %w", node, err)
		}
		element.Hash = nodeHash
	}

	return element, nil
}

// retryOnTooManyRequestsStatus defines retry condition for Too Many Requests response.
func retryOnTooManyRequestsStatus(res *resty.Response, err error) bool {
	return res.StatusCode() == http.StatusTooManyRequests
}
