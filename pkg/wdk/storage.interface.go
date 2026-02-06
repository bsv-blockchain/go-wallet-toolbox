package wdk

import (
	"context"
)

//go:generate go run -tags gen ../../tools/client-gen/main.go -out ../storage/client_gen.go
//go:generate go run -tags gen ../../tools/client-gen/main.go -out wallet_storage_interface_gen.go -skip-methods "GetSyncChunk,FindOrInsertSyncStateAuth,ProcessSyncChunk" -tmpl wallet_storage.tpl
//go:generate go run -tags gen ../../tools/client-gen/main.go -out ../storage/storage_manager_gen.go -skip-methods "MakeAvailable,SetActive,GetSyncChunk,FindOrInsertSyncStateAuth,ProcessSyncChunk" -tmpl manager.tpl
//go:generate go run -tags gen ../../tools/client-gen/main.go -out ../storage/internal/server/rpc_storage_provider.gen.go -tmpl rpc_storage_provider.tpl
//go:generate go tool mockgen -destination=../internal/mocks/mock_wallet_storage_writer.go -package=mocks github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk WalletStorageProvider

// WalletStorageProvider is an interface for writing to the wallet storage
type WalletStorageProvider interface {

	// Migrate migrates a wallet storage database.
	// @Write
	// @NonRPC
	Migrate(ctx context.Context, storageName string, storageIdentityKey string) (string, error)

	// MakeAvailable makes the storage available storage for user.
	// @Write
	MakeAvailable(ctx context.Context) (*TableSettings, error)

	// SetActive updates the active storage identity key for the authenticated user.
	// @Write
	SetActive(ctx context.Context, auth AuthID, newActiveStorageIdentityKey string) error

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

	// RelinquishOutput removes the specified output from the users outputs.
	// @Write
	RelinquishOutput(ctx context.Context, auth AuthID, args RelinquishOutputArgs) error

	// ListCertificates retrieves a paginated list of certificates based on the provided filter and pagination arguments.
	// @Read
	ListCertificates(ctx context.Context, auth AuthID, args ListCertificatesArgs) (*ListCertificatesResult, error)

	// ListOutputs retrieves a list of wallet outputs based on the provided query parameters in the arguments.
	// @Read
	ListOutputs(ctx context.Context, auth AuthID, args ListOutputsArgs) (*ListOutputsResult, error)

	// ListActions retrieves a list of wallet actions based on the provided query parameters in the arguments.
	// @Read
	ListActions(ctx context.Context, auth AuthID, args ListActionsArgs) (*ListActionsResult, error)

	// GetSyncChunk retrieves a chunk of sync data for a user between two storages using the provided synchronization arguments.
	// Skipped in WalletStorage interface and not exposed in StorageManager.
	// @Sync
	GetSyncChunk(ctx context.Context, args RequestSyncChunkArgs) (*SyncChunk, error)

	// FindOrInsertSyncStateAuth retrieves an existing sync state or inserts a new one based on the provided authentication and storage details.
	// Skipped in WalletStorage interface and not exposed in StorageManager.
	// @Sync
	FindOrInsertSyncStateAuth(ctx context.Context, auth AuthID, storageIdentityKey, storageName string) (*FindOrInsertSyncStateAuthResponse, error)

	// ProcessSyncChunk processes a sync chunk for a user, applying the changes contained within it.
	// Skipped in WalletStorage interface and not exposed in StorageManager.
	// @Sync
	ProcessSyncChunk(ctx context.Context, args RequestSyncChunkArgs, chunk *SyncChunk) (*ProcessSyncChunkResult, error)

	// AbortAction aborts a transaction that is in progress and has not yet been finalized or sent to the network.
	// @Write
	AbortAction(ctx context.Context, auth AuthID, args AbortActionArgs) (*AbortActionResult, error)

	// FindOutputBasketsAuth finds output baskets for the authenticated user based on the provided filters.
	// @Read
	FindOutputBasketsAuth(ctx context.Context, auth AuthID, filters FindOutputBasketsArgs) (TableOutputBaskets, error)

	// FindOutputsAuth finds outputs for the authenticated user based on the provided filters.
	// @Read
	FindOutputsAuth(ctx context.Context, auth AuthID, filters FindOutputsArgs) (TableOutputs, error)

	// ListTransactions retrieves a list of transactions with their status updates for the authenticated user.
	// @Read
	ListTransactions(ctx context.Context, auth AuthID, args ListTransactionsArgs) (*ListTransactionsResult, error)
}
