package wdk

import "time"

// RequestSyncChunkArgs contains parameters for requesting a chunk of sync data between two storage systems.
type RequestSyncChunkArgs struct {
	// FromStorageIdentityKey - The storageIdentityKey of the storage supplying the update SyncChunk data.
	FromStorageIdentityKey string `json:"fromStorageIdentityKey"`

	// ToStorageIdentityKey - The storageIdentityKey of the storage consuming the update SyncChunk data.
	ToStorageIdentityKey string `json:"toStorageIdentityKey"`

	// IdentityKey - The identity of whose data is being requested
	IdentityKey string `json:"identityKey"`

	// Since - The max updated_at time received from the storage service receiving the request.
	// Will be nil if this is the first request or if no data was previously sync'ed.
	// `since` must include items if 'updated_at' is greater or equal. Thus, when not undefined, a sync request should always return at least one item already seen.
	Since *time.Time `json:"since,omitempty"`

	// MaxRoughSize - A rough limit on how large the response should be.
	// The item that exceeds the limit is included and ends adding more items.
	MaxRoughSize uint64 `json:"maxRoughSize"`

	// MaxItems - The maximum number of items (records) to be returned.
	MaxItems uint64 `json:"maxItems"`

	// Offsets - For each entity in dependency order, the offset at which to start returning items from 'since'.
	Offsets []SyncOffsets `json:"offsets"`
}

// SyncOffsets represents the offset position for syncing a specific entity identified by its name.
// Used to track progress within ordered entities during synchronization processes.
// Helps determine where to resume fetching data for incremental sync tasks.
type SyncOffsets struct {
	Name   EntityName `json:"name"`
	Offset uint64     `json:"offset"`
}

// SyncChunk contains a slice of data to synchronize between storages for a particular user.
// It includes storage identity keys and chunks of entities.
// Used to transfer a consistent batch of data during synchronization operations between wallets or servers.
type SyncChunk struct {
	FromStorageIdentityKey string `json:"fromStorageIdentityKey"`
	ToStorageIdentityKey   string `json:"toStorageIdentityKey"`
	UserIdentityKey        string `json:"userIdentityKey"`

	User          *TableUser           `json:"user,omitempty"`
	OutputBaskets []*TableOutputBasket `json:"outputBaskets,omitempty"`
}
