package whatsonchain

import (
	"context"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func (woc *WhatsOnChain) getStatusForTxids(ctx context.Context, url string, txids []string) (*wdk.GetStatusForTxidsResult, error) {
	response, err := woc.doStatusRequest(ctx, url, txids)
	if err != nil {
		return nil, err
	}

	results := mapWocStatusResponse(response)

	return &wdk.GetStatusForTxidsResult{
		Name:    ServiceName,
		Status:  wdk.GetStatusSuccess,
		Results: results,
	}, nil
}
