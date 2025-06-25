package testutils

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
)

func SatoshiValue(p *wdk.StorageCreateTransactionSdkOutput) primitives.SatoshiValue {
	return p.Satoshis
}
