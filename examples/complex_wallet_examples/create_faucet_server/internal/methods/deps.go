package methods

import (
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type FaucetDeps struct {
	FaucetPrivateKey     *ec.PrivateKey
	Network              defs.BSVNetwork
	Storage              wdk.WalletStorageProvider
	MaxFaucetTotalAmount uint64 // 0 means unlimited
	Wallet               sdk.Interface
}
