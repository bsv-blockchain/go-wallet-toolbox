package whatsonchain

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
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

func (woc *WhatsOnChain) hashToHeader(ctx context.Context, blockHash string) (*wdk.MerklePathBlockHeader, error) {
	url, err := blockHeaderByHashURL(woc.url, blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to build block header URL for hash %s: %w", blockHash, err)
	}

	var hdrResp blockHeaderResponse
	res, err := woc.httpClient.R().
		SetContext(ctx).
		SetResult(&hdrResp).
		Get(url)
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

	return &wdk.MerklePathBlockHeader{
		Height:     height,
		Hash:       blockHash,
		MerkleRoot: hdrResp.MerkleRoot,
	}, nil
}

// getTscProof retrieves the TSC proof from WoC.
func (woc *WhatsOnChain) getTscProof(ctx context.Context, txID string) (*tscProof, error) {
	url, err := tscProofURL(woc.url, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to build TSC proof URL for txID %s: %w", txID, err)
	}
	var proofs []tscProof

	req := woc.httpClient.R().
		SetContext(ctx).
		SetResult(&proofs)
	res, err := req.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query TSC proof: %w", err)
	}
	if res.StatusCode() == http.StatusNotFound {
		return nil, nil
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching TSC proof", res.StatusCode())
	}

	if len(proofs) == 0 {
		return nil, nil
	}

	return &proofs[0], nil
}

// fetchAndCompare fetches the block header for the given height and compares its Merkle root with the provided root.
func (woc *WhatsOnChain) fetchAndCompare(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {
	url, err := blockHeaderURL(woc.url, height)
	if err != nil {
		return false, fmt.Errorf("failed to build block header URL for height %d: %w", height, err)
	}

	var dto struct {
		MerkleRoot string `json:"merkleroot"`
	}

	resp, err := woc.httpClient.R().
		SetContext(ctx).
		SetResult(&dto).
		AddRetryCondition(httpx.RetryOnErrOr5xx).
		Get(url)
	if err != nil {
		return false, fmt.Errorf("failed to fetch block header for height %d: %w", height, err)
	}

	switch resp.StatusCode() {
	case http.StatusOK:
	case http.StatusNotFound:
		woc.rootCache[height] = new(chainhash.Hash)
		return false, nil
	default:
		return false, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.Status())
	}

	remote, err := chainhash.NewHashFromHex(dto.MerkleRoot)
	if err != nil {
		return false, fmt.Errorf("failed to parse Merkle root %q for height %d: %w", dto.MerkleRoot, height, err)
	}

	woc.rootCache[height] = remote
	return remote.IsEqual(root), nil
}
