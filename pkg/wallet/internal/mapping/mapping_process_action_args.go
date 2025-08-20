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
		IsNewTx:    wdkArgs.IsNewTx,
		IsSendWith: wdkArgs.IsSendWith,
		IsNoSend:   wdkArgs.IsNoSend,
		IsDelayed:  wdkArgs.IsDelayed,
		//FIXME: For now it is not possible to have new transaction with "sendWith" option, because of two reasons:
		// 1. when the newTx is created without "noSendChange outputs", BEEF has several subject transactions and this is a problem for ARC service to broadcast it
		// 2. when the newTx is created with "noSendChange outputs" from previous noSend tx, validation fails because we don't accept (!args.IsNoSend && len(args.Options.NoSendChange) > 0)
		// -- this behavior is aligned with TS version of the wallet
		SendWith:  to.IfThen(wdkArgs.IsSendWith, wdkArgs.Options.SendWith).ElseThen(nil),
		TxID:      to.Ptr(primitives.TXIDHexString(txid.String())),
		RawTx:     tx.Bytes(),
		Reference: &reference,
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
