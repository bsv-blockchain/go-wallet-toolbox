package mapping

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func MapRelinquishOutputArgs(args sdk.RelinquishOutputArgs) wdk.RelinquishOutputArgs {
	return wdk.RelinquishOutputArgs{
		Basket: args.Basket,
		Output: args.Output.String(),
	}
}
