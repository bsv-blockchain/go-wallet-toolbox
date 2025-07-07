package bitails

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

type proofResponse struct {
	Index  int      `json:"index"`
	TxOrId string   `json:"txOrId"`
	Target string   `json:"target"`
	Nodes  []string `json:"nodes"`
}

// MerklePath fetches a Merkle-path proof for the given txID using Bitails’
func (b *Bitails) MerklePath(ctx context.Context, txID string) (*wdk.MerklePathResult, error) {
	proof, err := b.getTscProof(ctx, txID)
	if err != nil {
		return nil, err
	}
	if proof == nil {
		return &wdk.MerklePathResult{Name: ServiceName}, nil
	}

	header, err := b.hashToHeader(ctx, proof.Target)
	if err != nil {
		return nil, fmt.Errorf("bitails: %w", err)
	}

	txInfo, err := b.fetchTxInfo(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to resolve block height: %w", err)
	}
	header.Height, err = to.UInt32(txInfo.BlockHeight)
	if err != nil {
		return nil, fmt.Errorf("bitails: invalid block height %d for %s: %w",
			txInfo.BlockHeight, txID, err)
	}

	merklePath, err := txutils.ConvertTscProofToMerklePath(
		txID,
		proof.Index,
		proof.Nodes,
		header.Height,
	)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to convert TSC proof: %w", err)
	}

	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to compute merkle root: %w", err)
	}
	if merkleRoot != header.MerkleRoot {
		return nil, fmt.Errorf("bitails: merkle root mismatch (got %s, want %s) for txID %s in block %s", merkleRoot, header.MerkleRoot, txID, header.Hash)
	}

	return &wdk.MerklePathResult{
		Name:        ServiceName,
		MerklePath:  merklePath,
		BlockHeader: header,
		Notes:       wdk.Notes{{When: to.Ptr(time.Now()), What: "getMerklePathTSC"}},
	}, nil
}

func (b *Bitails) hashToHeader(ctx context.Context, blockHash string) (*wdk.MerklePathBlockHeader, error) {
	url, err := blockHeaderURL(b.url, blockHash)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Header string `json:"header"`
	}

	res, err := b.httpClient.R().
		SetContext(ctx).
		SetResult(&resp).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("bitails: failed to fetch block header: %w", err)
	}

	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("bitails: unexpected status code %d fetching block header", res.StatusCode())
	}

	raw, err := hex.DecodeString(resp.Header)
	if err != nil {
		return nil, fmt.Errorf("bitails: invalid hex in header: %w", err)
	}

	if len(raw) != BlockHeaderLength {
		return nil, fmt.Errorf("bitails: expected 80-byte block header, got %d", len(raw))
	}

	merkleRootHash, err := chainhash.NewHash(raw[MerkleRootOffset : MerkleRootOffset+MerkleRootLength])
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to extract merkle root: %w", err)
	}

	return &wdk.MerklePathBlockHeader{
		Hash:       blockHash,
		MerkleRoot: merkleRootHash.String(),
	}, nil
}

// getTscProof queries /tx/{txid}/proof/tsc and returns nil on 404.
func (b *Bitails) getTscProof(ctx context.Context, txID string) (*proofResponse, error) {
	url, err := tscProofURL(b.url, txID)
	if err != nil {
		return nil, err
	}
	var proof proofResponse
	res, err := b.httpClient.R().
		SetContext(ctx).
		SetResult(&proof).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to query TSC proof: %w", err)
	}
	if res.StatusCode() == http.StatusNotFound {
		return nil, nil
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("bitails: unexpected status %d fetching TSC proof", res.StatusCode())
	}
	return &proof, nil
}
