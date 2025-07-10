package bitails

import (
	"context"
	"fmt"

	"github.com/go-softwarelab/common/pkg/to"
)

type networkInfoResponse struct {
	Blocks uint64 `json:"blocks"`
}

// GetHeight contacts the Bitails API and returns the current best-chain height.
func (b *Bitails) GetHeight(ctx context.Context) (uint32, error) {
	url, err := buildURL(b.url, "network", "info")
	if err != nil {
		return 0, fmt.Errorf("bitails: failed to build height URL: %w", err)
	}

	var payload networkInfoResponse
	res, err := b.httpClient.
		R().
		SetContext(ctx).
		SetResult(&payload).
		Get(url)
	if err != nil {
		return 0, fmt.Errorf("bitails: height query failed: %w", err)
	}
	if res.StatusCode() != HTTPStatusOK {
		return 0, fmt.Errorf("bitails: unexpected HTTP %d for %s", res.StatusCode(), url)
	}
	if payload.Blocks == 0 {
		return 0, fmt.Errorf("bitails: API returned height 0")
	}

	height, err := to.UInt32(payload.Blocks)
	if err != nil {
		return 0, fmt.Errorf("bitails: invalid height %d in response: %w", payload.Blocks, err)
	}
	return height, nil
}
