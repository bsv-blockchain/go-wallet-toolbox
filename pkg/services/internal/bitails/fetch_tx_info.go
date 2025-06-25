package bitails

import (
	"context"
	"fmt"
)

type fetchInfoResponse struct {
	TxID        string  `json:"txid"`
	BlockHash   string  `json:"blockHash"`
	BlockHeight int64   `json:"blockHeight"`
	MerklePath  *string `json:"merklePath,omitempty"`
}

func (b *Bitails) fetchTxInfo(ctx context.Context, txid string) (*fetchInfoResponse, error) {
	url := fmt.Sprintf("%s"+FetchInfoEndpointFormat, b.url, txid)
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
