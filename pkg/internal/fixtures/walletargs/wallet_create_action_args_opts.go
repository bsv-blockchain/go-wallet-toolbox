package walletargs

import (
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
)

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
