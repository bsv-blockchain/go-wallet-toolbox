package whatsonchain

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
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

// MerklePath retrieves the merkle path for a transaction using WoC TSC proof.
func (woc *WhatsOnChain) MerklePath(ctx context.Context, txID string) (*wdk.MerklePathResult, error) {
	proof, err := woc.getTscProof(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get TSC proof: %w", err)
	}
	if proof == nil {
		// Proof not found
		return &wdk.MerklePathResult{
			Name:  ServiceName,
			Notes: history.New().GetMerklePathNotFound(ServiceName).Note().AsList(),
		}, nil
	}

	header, err := woc.hashToHeader(ctx, proof.Target)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block header: %w", err)
	}

	merklePath, err := txutils.ConvertTscProofToMerklePath(txID, proof.Index, proof.Nodes, header.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to convert proof for tx %s to merkle path: %w", txID, err)
	}

	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	if err != nil {
		return nil, fmt.Errorf("failed to compute merkle root: %w", err)
	}
	if merkleRoot != header.MerkleRoot {
		return nil, fmt.Errorf("computed merkle root %q does not match block header %q", merkleRoot, header.MerkleRoot)
	}

	return &wdk.MerklePathResult{
		Name:        ServiceName,
		MerklePath:  merklePath,
		BlockHeader: header,
		Notes:       history.New().GetMerklePathSuccess(ServiceName).Note().AsList(),
	}, nil
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
