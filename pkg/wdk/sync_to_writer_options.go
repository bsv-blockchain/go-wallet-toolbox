package wdk

import (
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/types"
)

const (
	// MaxSyncChunkSize defines the maximum number of bytes in a single sync chunk when syncing data to a writer.
	MaxSyncChunkSize = 10_000_000

	// MaxSyncItems defines the maximum number of items to sync in a single sync operation.
	MaxSyncItems = 1000
)

// SyncToWriterOptions configures sync operations when writing data, such as chunk size and item count per sync.
type SyncToWriterOptions struct {
	MaxSyncChunkSize uint64
	MaxSyncItems     uint64
	ReaderFactory    func() WalletStorageProvider
}

// SyncToWriterOption defines a function type for customizing options during sync operations to a writer storage.
type SyncToWriterOption = func(o *SyncToWriterOptions)

// WithMaxSyncChunkSize sets the maximum chunk size, in bytes, for each sync operation when writing to storage.
func WithMaxSyncChunkSize[T types.Number](size T) SyncToWriterOption {
	return func(o *SyncToWriterOptions) {
		o.MaxSyncChunkSize = must.ConvertToUInt64(size)
	}
}

// WithMaxSyncItems sets the maximum number of items to sync in a single operation for SyncToWriterOptions.
func WithMaxSyncItems[T types.Number](items T) SyncToWriterOption {
	return func(o *SyncToWriterOptions) {
		o.MaxSyncItems = must.ConvertToUInt64(items)
	}
}

// WithSyncReader specifies the reader storage provider to use when syncing data to a writer storage.
// If not provided, currently active storage will be used as the reader.
func WithSyncReader(reader WalletStorageProvider) SyncToWriterOption {
	return func(o *SyncToWriterOptions) {
		if reader == nil {
			return
		}

		o.ReaderFactory = func() WalletStorageProvider {
			return reader
		}
	}
}
