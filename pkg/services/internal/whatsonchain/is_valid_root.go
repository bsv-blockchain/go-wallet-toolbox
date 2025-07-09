package whatsonchain

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
)

// IsValidRootForHeight checks if the provided Merkle root is valid for the given block height.
func (woc *WhatsOnChain) IsValidRootForHeight(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {
	if cached, ok := woc.rootCache[height]; ok {
		return cached.IsEqual(root), nil
	}

	var lastErr error

	for range woc.rootForHeightRetries {
		if ctx.Err() != nil {
			return false, fmt.Errorf("context canceled while validating Merkle root for height %d: %w", height, ctx.Err())
		}
		ok, err := woc.fetchAndCompare(ctx, root, height)
		if err == nil {
			return ok, nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return false, fmt.Errorf("context canceled while validating Merkle root for height %d: %w", height, ctx.Err())
		case <-time.After(woc.rootForHeightRetryInterval):
		}
	}

	return false, fmt.Errorf("WoC: %w (after %d retries)", lastErr, woc.rootForHeightRetries)
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
