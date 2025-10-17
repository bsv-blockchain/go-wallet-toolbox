package whatsonchain

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/internal/dto"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func (woc *WhatsOnChain) ChainHeaderByHeight(ctx context.Context, height uint32) (*wdk.ChainBlockHeader, error) {
	var dst dto.BlockHeader
	req := woc.httpClient.
		R().
		SetContext(ctx).
		SetResult(&dst)

	res, err := req.Get(fmt.Sprintf("%s/block/%d/header", woc.url, height))
	if err != nil {
		return nil, fmt.Errorf("unexpected response from API (URL: %s): %w", req.URL, err)
	}

	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response from API (URL: %s, code: %d)", req.URL, res.StatusCode())
	}

	if dst.IsZero() {
		return nil, fmt.Errorf("expected a non-empty block header at height %d", height)
	}

	blockHeader, err := dst.ConvertToChainBlockHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to convert block header response by height from WoC to a chain base block header: %w", err)
	}
	return blockHeader, nil
}
