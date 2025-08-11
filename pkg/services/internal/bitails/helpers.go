package bitails

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/internal/dto"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// getTxStatus retrieves the status of a transaction by its txid from Bitails.
func (b *Bitails) getTxStatus(ctx context.Context, txid string) (found bool, mined bool, height uint32, err error) {
	url, err := txStatusURL(b.url, txid)
	if err != nil {
		err = fmt.Errorf("build tx status URL: %w", err)
		return
	}

	var info dto.FetchInfoResponse
	found, err = b.handleJSON(ctx, url, &info, http.StatusOK, true /* allow 404 */)
	if err != nil {
		err = fmt.Errorf("fetch tx status: %w", err)
		return
	}

	mined = info.BlockHeight > 0
	height = info.BlockHeight
	return
}

// fetchRemoteRoot retrieves the Merkle root for a given block height from Bitails.
func (b *Bitails) fetchRemoteRoot(ctx context.Context, height uint32) (*chainhash.Hash, error) {
	url, err := blockHeaderByHeightURL(b.url, height)
	if err != nil {
		return nil, fmt.Errorf("failed to build block-header URL: %w", err)
	}

	var dto struct {
		Header string `json:"header"`
	}

	resp, err := b.httpClient.R().
		SetContext(ctx).
		SetResult(&dto).
		AddRetryCondition(httpx.RetryOnErrOr5xx).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch header: %w", err)
	}

	switch resp.StatusCode() {
	case http.StatusOK:
		// continue
	case http.StatusNotFound:
		// DO NOT cache empty hash here
		return nil, nil
	default:
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.Status())
	}

	raw, err := hex.DecodeString(dto.Header)
	if err != nil {
		return nil, fmt.Errorf("decode header hex: %w", err)
	}
	if len(raw) != BlockHeaderLength {
		return nil, fmt.Errorf("want %d-byte header, got %d", BlockHeaderLength, len(raw))
	}

	hdr, err := ConvertHeader(raw, height)
	if err != nil {
		return nil, err
	}

	remoteRoot, err := chainhash.NewHashFromHex(hdr.MerkleRoot)
	if err != nil {
		return nil, fmt.Errorf("parse remote root: %w", err)
	}

	return remoteRoot, nil
}

// getTscProof queries /tx/{txid}/proof/tsc and returns nil on 404.
func (b *Bitails) getTscProof(ctx context.Context, txID string) (*dto.ProofResponse, error) {
	url, err := tscProofURL(b.url, txID)
	if err != nil {
		return nil, fmt.Errorf("error building TSC proof URL: %w", err)
	}

	var proof dto.ProofResponse
	found, err := b.handleJSON(ctx, url, &proof, http.StatusOK, true)
	if err != nil {
		return nil, fmt.Errorf("error handling TSC proof JSON: %w", err)
	}

	if !found {
		return nil, nil // 404 means no proof found
	}

	return &proof, nil
}

// fetchTxInfo retrieves transaction information from Bitails.
func (b *Bitails) fetchTxInfo(ctx context.Context, txid string) (*dto.FetchInfoResponse, error) {
	url, err := txStatusURL(b.url, txid)
	if err != nil {
		return nil, fmt.Errorf("error building transaction status URL: %w", err)
	}

	var info dto.FetchInfoResponse
	_, err = b.handleJSON(ctx, url, &info, http.StatusOK, false)
	if err != nil {
		return nil, fmt.Errorf("error fetching transaction info: %w", err)
	}
	return &info, nil
}

// latestBlock fetches the chain tip hash and height.
func (b *Bitails) latestBlock(ctx context.Context) (hash string, height uint32, err error) {
	url, err := latestBlockURL(b.url)
	if err != nil {
		return "", 0, fmt.Errorf("error building latest block URL: %w", err)
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
		return "", 0, fmt.Errorf("latest block hash empty")
	}
	return payload.Hash, payload.Height, nil
}

// rawHeader fetches and decodes the 80-byte block header.
func (b *Bitails) rawHeader(ctx context.Context, blockHash string) ([]byte, error) {
	url, err := blockHeaderURL(b.url, blockHash)
	if err != nil {
		return nil, fmt.Errorf("error building block header URL: %w", err)
	}

	var payload struct {
		Header string `json:"header"`
	}
	_, err = b.handleJSON(ctx, url, &payload, http.StatusOK, false)
	if err != nil {
		return nil, fmt.Errorf("error fetching block header: %w", err)
	}

	raw, err := hex.DecodeString(payload.Header)
	if err != nil {
		return nil, fmt.Errorf("error decoding block header hex: %w", err)
	}
	if len(raw) != BlockHeaderLength {
		return nil, fmt.Errorf("expected %d-byte block header, got %d bytes", BlockHeaderLength, len(raw))
	}
	return raw, nil
}

// fetchMerkleHeader converts a block hash to a MerklePathBlockHeader.
func (b *Bitails) fetchMerkleHeader(ctx context.Context, blockHash string) (*wdk.MerklePathBlockHeader, error) {
	raw, err := b.rawHeader(ctx, blockHash)
	if err != nil {
		return nil, fmt.Errorf("error fetching raw block header: %w", err)
	}

	merkleRootHash, err := chainhash.NewHash(raw[MerkleRootOffset : MerkleRootOffset+MerkleRootLength])
	if err != nil {
		return nil, fmt.Errorf("error parsing Merkle root from raw header: %w", err)
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
func (b *Bitails) handleJSON(ctx context.Context, url string, out any, okCode int, notFoundIsOK bool) (found bool, err error) {

	res, err := b.httpClient.R().SetContext(ctx).SetResult(out).Get(url)
	if err != nil {
		return false, fmt.Errorf("error performing GET %s: %w", url, err)
	}

	switch res.StatusCode() {
	case okCode:
		return true, nil
	case http.StatusNotFound:
		if notFoundIsOK {
			return false, nil
		}
		fallthrough
	default:
		return false, fmt.Errorf("unexpected HTTP status %d for %s: %s", res.StatusCode(), url, res.Status())
	}
}

func (b *Bitails) getRootFromCache(height uint32) (*chainhash.Hash, bool) {
	b.cacheMu.RLock()
	defer b.cacheMu.RUnlock()
	root, ok := b.rootCache[height]
	return root, ok
}

func (b *Bitails) storeRootInCache(height uint32, root *chainhash.Hash) {
	b.cacheMu.Lock()
	defer b.cacheMu.Unlock()
	b.rootCache[height] = root
}

func (b *Bitails) fetchScriptHistory(ctx context.Context, scriptHash string) ([]dto.ScriptHistoryItem, error) {
	url, err := scriptHashHistoryURL(b.url, scriptHash)
	if err != nil {
		return nil, fmt.Errorf("build URL for script history: %w", err)
	}

	var dst dto.ScriptHistoryResponse
	// NOTE: Although pagination is not supported, we still send a "limit" to match WoC behavior.
	req := b.httpClient.R().
		SetContext(ctx).
		SetResult(&dst).
		SetQueryParam("limit", fmt.Sprint(b.hashScriptHistoryPageLimit)).
		SetResult(&dst)

	res, err := req.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get script history: %w", err)
	}

	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching script history", res.StatusCode())
	}
	if dst.Error != "" {
		return nil, fmt.Errorf("error in script history response: %s", dst.Error)
	}

	return dst.History, nil
}
