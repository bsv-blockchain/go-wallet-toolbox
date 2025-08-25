package methods

import (
	"context"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
)

const (
	balanceListLimit = uint32(100)
)

// ComputeBalance pages through wallet outputs and returns the total satoshis.
func ComputeBalance(ctx context.Context, w WalletLister, basket string) (uint64, error) {
	var balance uint64
	var offset uint32

	for {
		args := sdk.ListOutputsArgs{
			Basket: basket,
			Limit:  to.Ptr(balanceListLimit),
			Offset: &offset,
		}

		outputs, err := w.ListOutputs(ctx, args, "")
		if err != nil {
			return 0, err
		}

		for _, output := range outputs.Outputs {
			balance += output.Satoshis
		}

		offset += uint32(len(outputs.Outputs))
		if len(outputs.Outputs) < int(balanceListLimit) {
			break
		}
	}

	return balance, nil
}

// WalletLister abstracts the subset of wallet needed for computing balance.
type WalletLister interface {
	ListOutputs(ctx context.Context, args sdk.ListOutputsArgs, originator string) (*sdk.ListOutputsResult, error)
}
