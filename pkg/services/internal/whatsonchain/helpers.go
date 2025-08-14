package whatsonchain

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/internal/dto"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

func (woc *WhatsOnChain) fetchMerkleHeader(ctx context.Context, blockHash string) (*wdk.MerklePathBlockHeader, error) {
	url, err := blockHeaderByHashURL(woc.url, blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to build block header URL for hash %s: %w", blockHash, err)
	}

	var hdrResp dto.BlockHeaderResponse
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
func (woc *WhatsOnChain) getTscProof(ctx context.Context, txID string) (*dto.TscProof, error) {
	url, err := tscProofURL(woc.url, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to build TSC proof URL for txID %s: %w", txID, err)
	}
	var proofs []dto.TscProof

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

// fetchRemoteRoot retrieves the Merkle root for the given block height from the WoC API.
func (woc *WhatsOnChain) fetchRemoteRoot(ctx context.Context, height uint32) (*chainhash.Hash, error) {
	url, err := blockHeaderURL(woc.url, height)
	if err != nil {
		return nil, fmt.Errorf("failed to build block header URL for height %d: %w", height, err)
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
		return nil, fmt.Errorf("failed to fetch block header for height %d: %w", height, err)
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

	remote, err := chainhash.NewHashFromHex(dto.MerkleRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Merkle root %q for height %d: %w", dto.MerkleRoot, height, err)
	}

	return remote, nil
}

func (woc *WhatsOnChain) getRootFromCache(height uint32) (*chainhash.Hash, bool) {
	woc.cacheMu.RLock()
	defer woc.cacheMu.RUnlock()
	val, ok := woc.rootCache[height]
	return val, ok
}

func (woc *WhatsOnChain) storeRootInCache(height uint32, root *chainhash.Hash) {
	woc.cacheMu.Lock()
	defer woc.cacheMu.Unlock()
	woc.rootCache[height] = root
}

func (woc *WhatsOnChain) doStatusRequest(ctx context.Context, url string, txids []string) (dto.WocStatusResponse, error) {
	var response dto.WocStatusResponse

	respWrapper := woc.httpClient.R().
		SetContext(ctx).
		SetBody(dto.WocStatusRequest{Txids: txids}).
		SetResult(&response).
		AddRetryCondition(httpx.RetryOnErrOr5xx)

	httpResp, err := respWrapper.Post(url)
	if err != nil {
		return nil, fmt.Errorf("request to WoC failed: %w", err)
	}
	if httpResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from WoC: %d", httpResp.StatusCode())
	}

	return response, nil
}

func (woc *WhatsOnChain) mapSingleTxStatus(tx dto.WocStatusItem) wdk.TxStatusDetail {
	var (
		depth  *int
		status wdk.ResultStatusForTxID
	)

	if tx.Error != nil {
		if *tx.Error != "unknown" {
			woc.logger.Warn("unexpected error for tx", slog.String("txid", tx.TxID), slog.String("error", *tx.Error))
		}
		status = wdk.ResultStatusForTxIDNotFound
		return wdk.TxStatusDetail{TxID: tx.TxID, Depth: nil, Status: status.String()}
	}

	if tx.Confirmations == nil {
		if tx.BlockHash != "" {
			woc.logger.Warn("blockhash present but confirmations=nil", slog.String("txid", tx.TxID), slog.String("blockhash", tx.BlockHash))
		}
		status = wdk.ResultStatusForTxIDKnown
		depth = to.Ptr(0)
		return wdk.TxStatusDetail{TxID: tx.TxID, Depth: depth, Status: status.String()}
	}

	if *tx.Confirmations <= 0 || (tx.BlockHash != "" && *tx.Confirmations == 0) {
		woc.logger.Warn("non-positive confirmations or blockhash with zero confirmations", slog.String("txid", tx.TxID), slog.String("blockhash", tx.BlockHash), slog.Int("confirmations", *tx.Confirmations))
	}

	status = wdk.ResultStatusForTxIDMined
	depth = to.Ptr(*tx.Confirmations)

	return wdk.TxStatusDetail{
		TxID:   tx.TxID,
		Depth:  depth,
		Status: status.String(),
	}
}
