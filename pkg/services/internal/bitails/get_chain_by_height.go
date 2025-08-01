package bitails

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/internal/dto"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func (b *Bitails) GetChainHeaderByHeight(ctx context.Context, height uint32) (*wdk.ChainBaseBlockHeader, error) {
	var dst dto.BlockHeaderByHeightDTO
	req := b.httpClient.
		R().
		SetContext(ctx).
		SetResult(&dst)

	res, err := req.Get(fmt.Sprintf("%s/block/height/%d", b.url, height))
	if err != nil {
		return nil, fmt.Errorf("unexpected response from API (URL: %s): %w", req.URL, err)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response from API (URL: %s, code: %d)", req.URL, res.StatusCode())
	}

	return dst.ConvertToChainBaseBlockHeader(), nil
}
