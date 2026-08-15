package actions

import (
	"context"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type WalletStorageCreateAndProcessAction interface {
	WalletStorageCreateAction
	WalletStorageProcessAction
	WalletStorageAbortAction
}

type WalletStorageCreateAction interface {
	CreateAction(ctx context.Context, args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, error)
}

type WalletStorageProcessAction interface {
	ProcessAction(ctx context.Context, args wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error)
}

// WalletStorageProcessAndAbortAction is used by flows that finish an already created
// action: they either process it, or release its reserved inputs when they cannot.
type WalletStorageProcessAndAbortAction interface {
	WalletStorageProcessAction
	WalletStorageAbortAction
}

// WalletStorageAbortAction is used to release a created-but-not-yet-processed action,
// so that the inputs it reserved become spendable again.
type WalletStorageAbortAction interface {
	AbortAction(ctx context.Context, args wdk.AbortActionArgs) (*wdk.AbortActionResult, error)
}
