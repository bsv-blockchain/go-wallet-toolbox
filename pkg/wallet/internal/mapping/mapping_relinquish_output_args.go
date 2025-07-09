package mapping

import (
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func MapRelinquishOutputArgs(args sdk.RelinquishOutputArgs) wdk.RelinquishOutputArgs {
	return wdk.RelinquishOutputArgs{
		Basket: args.Basket,
		Output: args.Output.String(),
	}
}
