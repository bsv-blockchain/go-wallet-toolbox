package wdk

import "context"

// WalletStorage represents a storage interface required by the wallet.
type WalletStorage interface {
	GetAuth(ctx context.Context) (AuthID, error)
	WalletStorageBasic
}
