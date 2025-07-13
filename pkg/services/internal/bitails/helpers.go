package bitails

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
)

// fetchAndCompareRoot fetches the raw 80-byte header for `height` from Bitails,
func (b *Bitails) fetchAndCompareRoot(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {

	url, err := blockHeaderByHeightURL(b.url, height)
	if err != nil {
		return false, fmt.Errorf("failed to build block-header URL: %w", err)
	}

	var dto struct {
		Header string `json:"header"`
	}

	resp, err := b.httpClient.R().SetContext(ctx).SetResult(&dto).AddRetryCondition(httpx.RetryOnErrOr5xx).Get(url)
	if err != nil {
		return false, fmt.Errorf("failed to fetch header: %w", err)
	}

	switch resp.StatusCode() {
	case http.StatusOK:
	case http.StatusNotFound:
		b.rootCache[height] = new(chainhash.Hash)
		return false, nil
	default:
		return false, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.Status())
	}

	raw, err := hex.DecodeString(dto.Header)
	if err != nil {
		return false, fmt.Errorf("decode header hex: %w", err)
	}
	if len(raw) != BlockHeaderLength {
		return false, fmt.Errorf("want %d-byte header, got %d", BlockHeaderLength, len(raw))
	}

	hdr, err := ConvertHeader(raw, height)
	if err != nil {
		return false, err
	}

	remoteRoot, err := chainhash.NewHashFromHex(hdr.MerkleRoot)
	if err != nil {
		return false, fmt.Errorf("parse remote root: %w", err)
	}

	b.rootCache[height] = remoteRoot
	return remoteRoot.IsEqual(root), nil
}
