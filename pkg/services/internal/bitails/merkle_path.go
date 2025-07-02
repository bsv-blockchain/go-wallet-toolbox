package bitails

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/go-softwarelab/common/pkg/to"
)

type proofResponse struct {
	Index  int      `json:"index"`
	TxOrId string   `json:"txOrId"`
	Target string   `json:"target"`
	Nodes  []string `json:"nodes"`
}

type branchProofResponse struct {
	BlockHash  string         `json:"blockhash"`
	MerkleRoot string         `json:"merkleRoot"`
	Branches   []merkleBranch `json:"branches"`
}

type BranchPos string

const (
	Left  BranchPos = "L"
	Right BranchPos = "R"
)

type merkleBranch struct {
	Hash string    `json:"hash"`
	Pos  BranchPos `json:"pos"`
}

// MerklePath fetches a Merkle-path proof for the given txID using Bitails’
// /tx/{txid}/proof/tsc endpoint and returns it in wdk.MerklePathResult form.
func (b *Bitails) MerklePath(ctx context.Context, txID string) (*wdk.MerklePathResult, error) {
	// ── 1. Query TSC proof ──────────────────────────────────────────────
	proof, err := b.getTscProof(ctx, txID)
	if err != nil {
		return nil, err
	}
	if proof == nil { // tx not yet mined (404), nothing to return
		return &wdk.MerklePathResult{Name: ServiceName}, nil
	}

	// ── 2. Pull block header (target == block hash) ─────────────────────
	header, err := b.hashToHeader(ctx, proof.Target)
	if err != nil {
		return nil, fmt.Errorf("bitails: %w", err)
	}

	// Look up height (Bitails TSC proof doesn’t contain it)
	txInfo, err := b.fetchTxInfo(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to resolve block height: %w", err)
	}
	header.Height, err = to.UInt32(txInfo.BlockHeight)
	if err != nil {
		return nil, fmt.Errorf("bitails: invalid block height %d for %s: %w",
			txInfo.BlockHeight, txID, err)
	}

	// ── 3. Convert proof → MerklePath ───────────────────────────────────
	merklePath, err := txutils.ConvertTscProofToMerklePath(
		txID,
		proof.Index,
		proof.Nodes,
		header.Height,
	)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to convert TSC proof: %w", err)
	}

	// ── 4. Sanity-check merkle root ─────────────────────────────────────
	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to compute merkle root: %w", err)
	}
	if merkleRoot != header.MerkleRoot {
		return nil, fmt.Errorf("bitails: merkle root mismatch (got %s, want %s)",
			merkleRoot, header.MerkleRoot)
	}

	// ── 5. Package result ───────────────────────────────────────────────
	return &wdk.MerklePathResult{
		Name:        ServiceName,
		MerklePath:  merklePath,
		BlockHeader: header,
		Notes:       wdk.Notes{{When: to.Ptr(time.Now()), What: "getMerklePathTSC"}},
	}, nil
}

func (b *Bitails) hashToHeader(ctx context.Context, blockHash string) (*wdk.MerklePathBlockHeader, error) {
	var resp struct {
		Header string `json:"header"`
	}

	res, err := b.httpClient.R().
		SetContext(ctx).
		SetResult(&resp).
		Get(fmt.Sprintf("%sblock/%s/header", b.url, blockHash))

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
	var proof proofResponse
	res, err := b.httpClient.R().
		SetContext(ctx).
		SetResult(&proof).
		Get(fmt.Sprintf("%stx/%s/proof/tsc", b.url, txID))
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to query TSC proof: %w", err)
	}
	if res.StatusCode() == http.StatusNotFound {
		return nil, nil // not in a block yet (or service lagging)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("bitails: unexpected status %d fetching TSC proof", res.StatusCode())
	}
	return &proof, nil
}
