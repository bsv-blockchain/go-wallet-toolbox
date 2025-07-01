package bitails

import (
	"context"
	"fmt"
)

type fetchInfoResponse struct {
	TxID        string `json:"txid"`
	BlockHash   string `json:"blockhash"`
	BlockHeight int64  `json:"blockheight"`
}

func (b *Bitails) fetchTxInfo(ctx context.Context, txid string) (*fetchInfoResponse, error) {
	url, err := buildTxStatusURL(b.url, txid)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL for fetching tx info: %w", err)
	}
	var resp fetchInfoResponse

	r, err := b.httpClient.R().
		SetContext(ctx).
		SetResult(&resp).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("%s fetch status failed: %w", ServiceName, err)
	}
	if r.StatusCode() != HTTPStatusOK {
		return nil, fmt.Errorf("%s fetch status unexpected HTTP %d", ServiceName, r.StatusCode())
	}
	return &resp, nil
}
