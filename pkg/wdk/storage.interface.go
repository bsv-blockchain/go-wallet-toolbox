package wdk

import (
	"context"
)

//go:generate go run -tags gen ../../tools/client-gen/main.go -out ../storage/client_gen.go
//go:generate go tool mockgen -destination=../internal/mocks/mock_wallet_storage_writer.go -package=mocks github.com/4chain-ag/go-wallet-toolbox/pkg/wdk WalletStorageWriter

// WalletStorageWriter is an interface for writing to the wallet storage
type WalletStorageWriter interface {
	Migrate(ctx context.Context, storageName string, storageIdentityKey string) (string, error)
	MakeAvailable(ctx context.Context) (*TableSettings, error)
	FindOrInsertUser(ctx context.Context, identityKey string) (*FindOrInsertUserResponse, error)
	InternalizeAction(ctx context.Context, auth AuthID, args InternalizeActionArgs) (*InternalizeActionResult, error)
	CreateAction(ctx context.Context, auth AuthID, args ValidCreateActionArgs) (*StorageCreateActionResult, error)
	ProcessAction(ctx context.Context, auth AuthID, args ProcessActionArgs) (*ProcessActionResult, error)

	InsertCertificateAuth(ctx context.Context, auth AuthID, certificate *TableCertificateX) (uint, error)
	RelinquishCertificate(ctx context.Context, auth AuthID, args RelinquishCertificateArgs) error
	ListCertificates(ctx context.Context, auth AuthID, args ListCertificatesArgs) (*ListCertificatesResult, error)
	ListOutputs(ctx context.Context, auth AuthID, args ListOutputsArgs) (*ListOutputsResult, error)
}
