package walletargs

import (
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
)

func WithLockingScript(lockingScript script.Script) func(args *wallet.CreateActionArgs) {
	return func(args *wallet.CreateActionArgs) {
		args.Outputs[0].LockingScript = lockingScript
		args.Outputs[0].CustomInstructions = ""
	}
}

func WithNoOutputs() func(args *wallet.CreateActionArgs) {
	return func(args *wallet.CreateActionArgs) {
		args.Outputs = nil
	}
}

func WithInputBEEF(inputBEEF []byte) func(args *wallet.CreateActionArgs) {
	return func(args *wallet.CreateActionArgs) {
		args.InputBEEF = inputBEEF
	}
}

func WithInputs(inputs []wallet.CreateActionInput) func(args *wallet.CreateActionArgs) {
	return func(args *wallet.CreateActionArgs) {
		args.Inputs = inputs
	}
}

func WithInput(inputSource CreateActionInputSource) func(args *wallet.CreateActionArgs) {
	return func(args *wallet.CreateActionArgs) {
		args.InputBEEF = inputSource.InputBEEFBytes()
		args.Inputs = []wallet.CreateActionInput{
			inputSource.CreateActionInput(),
		}
	}
}

func WithSignAndProcess(signAndProcess bool) func(args *wallet.CreateActionArgs) {
	return func(args *wallet.CreateActionArgs) {
		args.Options.SignAndProcess = to.Ptr(signAndProcess)
	}
}

func WithNoSend(noSend bool) func(args *wallet.CreateActionArgs) {
	return func(args *wallet.CreateActionArgs) {
		args.Options.NoSend = to.Ptr(noSend)
	}
}

func WithSendWith(sendWith ...chainhash.Hash) func(args *wallet.CreateActionArgs) {
	return func(args *wallet.CreateActionArgs) {
		args.Options.SendWith = sendWith
	}
}

func WithDelayedBroadcast() func(args *wallet.CreateActionArgs) {
	return func(args *wallet.CreateActionArgs) {
		args.Options.AcceptDelayedBroadcast = to.Ptr(true)
	}
}

func WithSatoshisAsFirstOutput(satoshis uint64) func(args *wallet.CreateActionArgs) {
	return func(args *wallet.CreateActionArgs) {
		if len(args.Outputs) == 0 {
			panic("no provided outputs")
		}
		args.Outputs[0].Satoshis = satoshis
	}
}

func WithNoSendChangeOutputs(changeOutputs ...transaction.Outpoint) func(args *wallet.CreateActionArgs) {
	return func(args *wallet.CreateActionArgs) {
		args.Options.NoSendChange = changeOutputs
	}
}

func WithoutProvidedOutputs() func(args *wallet.CreateActionArgs) {
	return func(args *wallet.CreateActionArgs) {
		args.Outputs = nil
	}
}
