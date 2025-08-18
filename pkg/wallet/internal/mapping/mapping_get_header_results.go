package mapping

import (
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func MapGetHeaderResults(results *wdk.ChainBaseBlockHeader) (*sdk.GetHeaderResult, error) {
	if results == nil {
		return nil, fmt.Errorf("results must not be nil")
	}

	header, err := results.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to convert ChainBaseBlockHeader to bytes: %w", err)
	}

	return &sdk.GetHeaderResult{
		Header: header,
	}, nil
}
