package whatsonchain

import (
	"context"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
)

func (woc *WhatsOnChain) getStatusForTxIDs(ctx context.Context, url string, txIDs []string) (*wdk.GetStatusForTxIDsResult, error) {
	response, err := woc.doStatusRequest(ctx, url, txIDs)
	if err != nil {
		return nil, err
	}

	results := slices.Map(response, woc.mapSingleTxStatus)

	return &wdk.GetStatusForTxIDsResult{
		Name:    ServiceName,
		Status:  wdk.GetStatusSuccess,
		Results: results,
	}, nil
}
