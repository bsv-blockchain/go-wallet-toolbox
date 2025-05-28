package wdk

import (
	"context"
)

//go:generate go run -tags gen ../../tools/client-gen/main.go -out ../storage/client_gen.go
//go:generate go run -tags gen ../../tools/client-gen/main.go -out wallet_storage_interface_gen.go -tmpl wallet_storage.tpl
//go:generate go run -tags gen ../../tools/client-gen/main.go -out ../storage/storage_manager_gen.go -skip-methods "MakeAvailable" -tmpl manager.tpl
//go:generate go tool mockgen -destination=../internal/mocks/mock_wallet_storage_writer.go -package=mocks github.com/4chain-ag/go-wallet-toolbox/pkg/wdk WalletStorageProvider

// WalletStorageProvider is an interface for writing to the wallet storage
type WalletStorageProvider interface {

	// Migrate migrates a wallet storage database.
	// @Write
	Migrate(ctx context.Context, storageName string, storageIdentityKey string) (string, error)

	// MakeAvailable makes the storage available storage for user.
	// @Write
	MakeAvailable(ctx context.Context) (*TableSettings, error)

	// FindOrInsertUser retrieves an existing user or inserts a new one based on the given identity key.
	// @Write
	FindOrInsertUser(ctx context.Context, identityKey string) (*FindOrInsertUserResponse, error)

	// InternalizeAction handles the internalization of a transaction from the outside of the wallet.
	// @Write
	InternalizeAction(ctx context.Context, auth AuthID, args InternalizeActionArgs) (*InternalizeActionResult, error)

	// CreateAction creates a new transaction ready to be signed and processed later.
	// @Write
	CreateAction(ctx context.Context, auth AuthID, args ValidCreateActionArgs) (*StorageCreateActionResult, error)

	// ProcessAction processes a signed transaction created by CreateAction.
	// @Write
	ProcessAction(ctx context.Context, auth AuthID, args ProcessActionArgs) (*ProcessActionResult, error)

	// InsertCertificateAuth adds a new certificate for a user.
	// @Write
	InsertCertificateAuth(ctx context.Context, auth AuthID, certificate *TableCertificateX) (uint, error)

	// RelinquishCertificate revokes the specified certificate from the users certificates.
	// @Write
	RelinquishCertificate(ctx context.Context, auth AuthID, args RelinquishCertificateArgs) error

	// ListCertificates retrieves a paginated list of certificates based on the provided filter and pagination arguments.
	// @Read
	ListCertificates(ctx context.Context, auth AuthID, args ListCertificatesArgs) (*ListCertificatesResult, error)

	// ListOutputs retrieves a list of wallet outputs based on the provided query parameters in the arguments.
	// @Read
	ListOutputs(ctx context.Context, auth AuthID, args ListOutputsArgs) (*ListOutputsResult, error)
}
