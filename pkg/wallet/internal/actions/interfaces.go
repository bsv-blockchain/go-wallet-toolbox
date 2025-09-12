package actions

import (
	"context"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type WalletStorageCreateAndProcessAction interface {
	WalletStorageCreateAction
	WalletStorageProcessAction
}

type WalletStorageCreateAction interface {
	CreateAction(ctx context.Context, args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, error)
}

type WalletStorageProcessAction interface {
	ProcessAction(ctx context.Context, args wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error)
}
