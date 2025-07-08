package testutils

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func SatoshiValue(p *wdk.StorageCreateTransactionSdkOutput) primitives.SatoshiValue {
	return p.Satoshis
}
