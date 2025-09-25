package mapping

import (
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
)

func MapProcessActionArgsForNewTx(txid *chainhash.Hash, tx *assembler.AssembledTransaction, reference string, wdkArgs wdk.ValidCreateActionArgs) wdk.ProcessActionArgs {
	processActionArgs := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: wdkArgs.IsSendWith,
		IsNoSend:   wdkArgs.IsNoSend,
		IsDelayed:  wdkArgs.IsDelayed,
		SendWith:   to.IfThen(wdkArgs.IsSendWith, wdkArgs.Options.SendWith).ElseThen([]primitives.TXIDHexString{}),
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
		SendWith:   to.IfThen(wdkArgs.Options.SendWith != nil, wdkArgs.Options.SendWith).ElseThen([]primitives.TXIDHexString{}),
		IsSendWith: true,
	}
	return processActionArgs
}
