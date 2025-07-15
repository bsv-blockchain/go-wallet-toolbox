package bitails

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// proofResponse defines the structure of the response from Bitails for TSC proofs.
type proofResponse struct {
	Index  int      `json:"index"`
	TxOrId string   `json:"txOrId"`
	Target string   `json:"target"`
	Nodes  []string `json:"nodes"`
}

// getTscProof queries /tx/{txid}/proof/tsc and returns nil on 404.
func (b *Bitails) getTscProof(ctx context.Context, txID string) (*proofResponse, error) {
	url, err := tscProofURL(b.url, txID)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s: to build URL for TSC proof: %w", ServiceName, err)
	}

	var proof proofResponse
	found, err := b.handleJSON(ctx, url, &proof, http.StatusOK, true)
	if err != nil || !found {
		return nil, fmt.Errorf("failed for service %s: get TSC proof: %w", ServiceName, err)
	}
	return &proof, nil
}

// fetchInfoResponse is the structure for the response from Bitails when fetching transaction info.
type fetchInfoResponse struct {
	TxID        string `json:"txid"`
	BlockHash   string `json:"blockhash"`
	BlockHeight int64  `json:"blockheight"`
}

// fetchTxInfo retrieves transaction information from Bitails.
func (b *Bitails) fetchTxInfo(ctx context.Context, txid string) (*fetchInfoResponse, error) {
	url, err := txStatusURL(b.url, txid)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s: build tx-status URL: %w", ServiceName, err)
	}

	var info fetchInfoResponse
	_, err = b.handleJSON(ctx, url, &info, http.StatusOK, false)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s: fetch tx info: %w", ServiceName, err)
	}
	return &info, nil
}

// latestBlock fetches the chain tip hash and height.
func (b *Bitails) latestBlock(ctx context.Context) (hash string, height uint32, err error) {
	url, err := latestBlockURL(b.url)
	if err != nil {
		return "", 0, fmt.Errorf("failed for service %s: build latest block URL: %w", ServiceName, err)
	}

	var payload struct {
		Hash   string `json:"hash"`
		Height uint32 `json:"height"`
	}
	_, err = b.handleJSON(ctx, url, &payload, http.StatusOK, false)
	if err != nil {
		return "", 0, err
	}
	if payload.Hash == "" {
		return "", 0, fmt.Errorf("failed for service %s: latest block hash empty", ServiceName)
	}
	return payload.Hash, payload.Height, nil
}

// rawHeader fetches and decodes the 80-byte block header.
func (b *Bitails) rawHeader(ctx context.Context, blockHash string) ([]byte, error) {
	url, err := blockHeaderURL(b.url, blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s: build block header URL: %w", ServiceName, err)
	}

	var payload struct {
		Header string `json:"header"`
	}
	_, err = b.handleJSON(ctx, url, &payload, http.StatusOK, false)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s: get block header: %w", ServiceName, err)
	}

	raw, err := hex.DecodeString(payload.Header)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s: header hex: %w", ServiceName, err)
	}
	if len(raw) != BlockHeaderLength {
		return nil, fmt.Errorf("failed for service %s: want %d-byte header, got %d", ServiceName, BlockHeaderLength, len(raw))
	}
	return raw, nil
}

// hashToHeader converts a block hash to a MerklePathBlockHeader.
func (b *Bitails) hashToHeader(ctx context.Context, blockHash string) (*wdk.MerklePathBlockHeader, error) {
	raw, err := b.rawHeader(ctx, blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s: get raw header: %w", ServiceName, err)
	}

	merkleRootHash, err := chainhash.NewHash(raw[MerkleRootOffset : MerkleRootOffset+MerkleRootLength])
	if err != nil {
		return nil, fmt.Errorf("failed for service %s: decode Merkle root: %w", ServiceName, err)
	}

	return &wdk.MerklePathBlockHeader{
		Hash:       blockHash,
		MerkleRoot: merkleRootHash.String(),
	}, nil
}

// handleJSON performs a GET, unmarshals JSON into 'out' and validates status.
//
//	okCode       - the HTTP status you expect (usually 200)
//	allow404     - if true, 404 is not an error (caller handles the "not found" case)
//
// It returns:
//
//	found = false   when allow404=true and the server returned 404
//	found = true    otherwise
func (b *Bitails) handleJSON(ctx context.Context, url string, out any, okCode int, allow404 bool) (found bool, err error) {

	res, err := b.httpClient.R().SetContext(ctx).SetResult(out).Get(url)
	if err != nil {
		return false, fmt.Errorf("%s: GET %s: %w", ServiceName, url, err)
	}

	switch res.StatusCode() {
	case okCode:
		return true, nil
	case http.StatusNotFound:
		if allow404 {
			return false, nil
		}
		fallthrough
	default:
		return false, fmt.Errorf("failed for service %s: unexpected HTTP %d for %s", ServiceName, res.StatusCode(), url)
	}
}
