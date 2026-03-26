package whatsonchain

import (
	"context"
	"slices"

	wdkSlices "github.com/go-softwarelab/common/pkg/slices"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func (woc *WhatsOnChain) getStatusForTxIDs(ctx context.Context, url string, txIDs []string) (*wdk.GetStatusForTxIDsResult, error) {
	chunks := slices.Collect(slices.Chunk(txIDs, 20))
	responses := make([]wdk.TxStatusDetail, 0, len(txIDs))

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
