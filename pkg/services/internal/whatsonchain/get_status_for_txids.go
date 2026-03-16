package whatsonchain

import (
	"context"
	"slices"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	wdkSlices "github.com/go-softwarelab/common/pkg/slices"
)

func (woc *WhatsOnChain) getStatusForTxIDs(ctx context.Context, url string, txIDs []string) (*wdk.GetStatusForTxIDsResult, error) {
	chunks := slices.Collect(slices.Chunk(txIDs, 20))
	responses := make([]wdk.TxStatusDetail, len(txIDs))

	for _, chunk := range chunks {
		response, err := woc.doStatusRequest(ctx, url, chunk)
		if err != nil {
			return nil, err
		}

		results := wdkSlices.Map(response, woc.mapSingleTxStatus)
		responses = append(responses, results...)
	}

	return &wdk.GetStatusForTxIDsResult{
		Name:    ServiceName,
		Status:  wdk.GetStatusSuccess,
		Results: responses,
	}, nil
}
