package mapping

import (
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
)

func MapProcessActionArgsForNewTx(txid *chainhash.Hash, tx *assembler.AssembledTransaction, reference string, wdkArgs wdk.ValidCreateActionArgs) wdk.ProcessActionArgs {
	sendWith := []primitives.TXIDHexString{}
	if wdkArgs.IsSendWith {
		sendWith = wdkArgs.Options.SendWith
	}

	processActionArgs := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: wdkArgs.IsSendWith,
		IsNoSend:   wdkArgs.IsNoSend,
		IsDelayed:  wdkArgs.IsDelayed,
		SendWith:   sendWith,
		TxID:       to.Ptr(primitives.TXIDHexString(txid.String())),
		RawTx:      tx.Bytes(),
		Reference:  &reference,
	}

	return processActionArgs
}

func MapProcessActionArgsForSendWith(wdkArgs wdk.ValidCreateActionArgs) wdk.ProcessActionArgs {
	sendWith := wdkArgs.Options.SendWith
	if sendWith == nil {
		sendWith = []primitives.TXIDHexString{}
	}

	processActionArgs := wdk.ProcessActionArgs{
		IsNewTx:    false,
		IsNoSend:   false,
		SendWith:   sendWith,
		IsSendWith: true,
	}
	return processActionArgs
}
