package mapping

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
)

func MapProcessActionArgs(txid *chainhash.Hash, tx *transaction.Transaction, reference string, wdkArgs wdk.ValidCreateActionArgs) wdk.ProcessActionArgs {
	processActionArgs := wdk.ProcessActionArgs{
		IsNewTx:    wdkArgs.IsNewTx,
		IsSendWith: wdkArgs.IsSendWith,
		IsNoSend:   wdkArgs.IsNoSend,
		IsDelayed:  wdkArgs.IsDelayed,
		SendWith:   to.IfThen(wdkArgs.IsSendWith, wdkArgs.Options.SendWith).ElseThen(nil),
	}

	processActionArgs.TxID = to.Ptr(primitives.TXIDHexString(txid.String()))
	processActionArgs.RawTx = tx.Bytes()

	processActionArgs.Reference = &reference
	return processActionArgs
}
