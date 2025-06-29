package bitails

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

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
	BlockHash  string `json:"blockhash"`
	MerkleRoot string `json:"merkleRoot"`
	Branches   []struct {
		Pos  string `json:"pos"`
		Hash string `json:"hash"`
	} `json:"branches"`
}

type merkleBranch struct {
	Hash string `json:"hash"`
	Pos  string `json:"pos"`
}

func (bitails *Bitails) MerklePath(ctx context.Context, txID string) (*wdk.MerklePathResult, error) {
	url := fmt.Sprintf("%stx/%s/proof/tsc", bitails.url, txID)

	var proof proofResponse
	resp, err := bitails.httpClient.R().
		SetContext(ctx).
		SetResult(&proof).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("bitails: failed to fetch TSC merkle proof: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return bitails.merklePathFromBranchProof(ctx, txID)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("bitails: unexpected status %d fetching TSC proof", resp.StatusCode())
	}

	txInfo, err := bitails.fetchTxInfo(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to resolve block height: %w", err)
	}
	blockHeight, err := to.UInt32(txInfo.BlockHeight)
	if err != nil {
		return nil, fmt.Errorf("bitails: invalid block height %d for transaction %s: %w", txInfo.BlockHeight, txID, err)
	}

	header, err := bitails.hashToHeader(ctx, proof.Target)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to fetch block header: %w", err)
	}
	header.Height = blockHeight

	merklePath, err := convertTscProofToMerklePath(txID, proof.Index, proof.Nodes, blockHeight)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to convert TSC proof: %w", err)
	}

	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to compute merkle root: %w", err)
	}

	if merkleRoot != header.MerkleRoot {
		return nil, fmt.Errorf("bitails: merkle root mismatch: computed %s, expected %s", merkleRoot, header.MerkleRoot)
	}

	return &wdk.MerklePathResult{
		Name:        ServiceName,
		MerklePath:  merklePath,
		BlockHeader: header,
		Notes:       wdk.Notes{{When: to.Ptr(time.Now()), What: "getMerklePathSuccess"}},
	}, nil
}

func (bitails *Bitails) merklePathFromBranchProof(ctx context.Context, txID string) (*wdk.MerklePathResult, error) {
	url := fmt.Sprintf("%s/tx/%s/proof", bitails.url, txID)

	var proof branchProofResponse
	resp, err := bitails.httpClient.R().
		SetContext(ctx).
		SetResult(&proof).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("bitails: failed to fetch branch merkle proof: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("bitails: unexpected status %d fetching branch proof", resp.StatusCode())
	}

	header, err := bitails.hashToHeader(ctx, proof.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to fetch block header: %w", err)
	}

	txInfo, err := bitails.fetchTxInfo(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to resolve block height: %w", err)
	}
	blockHeight, err := to.UInt32(txInfo.BlockHeight)
	if err != nil {
		return nil, fmt.Errorf("bitails: invalid block height %d for transaction %s: %w", txInfo.BlockHeight, txID, err)
	}
	header.Height = blockHeight

	branches := make([]merkleBranch, len(proof.Branches))
	for i, b := range proof.Branches {
		branches[i] = merkleBranch{Hash: b.Hash, Pos: b.Pos}
	}

	merklePath, err := convertBranchProofToMerklePath(txID, branches, header.Height)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to convert branch proof: %w", err)
	}

	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	if err != nil {
		return nil, fmt.Errorf("bitails: failed to compute merkle root: %w", err)
	}

	if merkleRoot != header.MerkleRoot {
		return nil, fmt.Errorf("bitails: merkle root mismatch: computed %s, expected %s", merkleRoot, header.MerkleRoot)
	}

	return &wdk.MerklePathResult{
		Name:        ServiceName,
		MerklePath:  merklePath,
		BlockHeader: header,
		Notes:       wdk.Notes{{When: to.Ptr(time.Now()), What: "getMerklePathFallback"}},
	}, nil
}

func (bitails *Bitails) hashToHeader(ctx context.Context, blockHash string) (*wdk.MerklePathBlockHeader, error) {
	var resp struct {
		Header string `json:"header"`
	}

	res, err := bitails.httpClient.R().
		SetContext(ctx).
		SetResult(&resp).
		Get(fmt.Sprintf("%sblock/%s/header", bitails.url, blockHash))

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
		// Height will be filled externally when txID is available
	}, nil
}
