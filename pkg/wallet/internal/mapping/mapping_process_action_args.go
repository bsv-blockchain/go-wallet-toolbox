package mapping

import (
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
)

func MapProcessActionArgsForNewTx(txid *chainhash.Hash, tx *transaction.Transaction, reference string, wdkArgs wdk.ValidCreateActionArgs) wdk.ProcessActionArgs {
	processActionArgs := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: wdkArgs.IsSendWith,
		IsNoSend:   wdkArgs.IsNoSend,
		IsDelayed:  wdkArgs.IsDelayed,
		SendWith:   to.IfThen(wdkArgs.IsSendWith, wdkArgs.Options.SendWith).ElseThen(nil),
		TxID:       to.Ptr(primitives.TXIDHexString(txid.String())),
		RawTx:      tx.Bytes(),
		Reference:  &reference,
	}

	return processActionArgs
}

func MapProcessActionArgsForSendWith(wdkArgs wdk.ValidCreateActionArgs) wdk.ProcessActionArgs {
	processActionArgs := wdk.ProcessActionArgs{
		IsNewTx:    false,
		IsNoSend:   false,
		SendWith:   wdkArgs.Options.SendWith,
		IsSendWith: true,
	}
	return processActionArgs
}
