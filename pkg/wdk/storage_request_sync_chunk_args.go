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

	User *TableUser `json:"user,omitempty"`

	// ATTENTION: The TS version keeps loading chunks (infinite-loop) if at least one of the entities is "undefined"
	// That's why `omitempty` is not used below and the slices are pre-initialized in NewSyncChunk.
	OutputBaskets     []*TableOutputBasket     `json:"outputBaskets"`
	ProvenTxs         []*TableProvenTx         `json:"provenTxs"`
	ProvenTxReqs      []*TableProvenTxReq      `json:"provenTxReqs"`
	TxLabels          []*TableTxLabel          `json:"txLabels"`
	OutputTags        []*TableOutputTag        `json:"outputTags"`
	Transactions      []*TableTransaction      `json:"transactions"`
	Outputs           []*TableOutput           `json:"outputs"`
	TxLabelMaps       []*TableTxLabelMap       `json:"txLabelMaps"`
	OutputTagMaps     []*TableOutputTagMap     `json:"outputTagMaps"`
	Certificates      []*TableCertificate      `json:"certificates"`
	CertificateFields []*TableCertificateField `json:"certificateFields"`
	Commissions       []*TableCommission       `json:"commissions"`
}

// NewSyncChunk creates a new SyncChunk with provided storage and user identity keys and initializes entity slices.
func NewSyncChunk(fromStorageIdentityKey, toStorageIdentityKey, userIdentityKey string) *SyncChunk {
	return &SyncChunk{
		FromStorageIdentityKey: fromStorageIdentityKey,
		ToStorageIdentityKey:   toStorageIdentityKey,
		UserIdentityKey:        userIdentityKey,

		OutputBaskets:     make([]*TableOutputBasket, 0),
		ProvenTxs:         make([]*TableProvenTx, 0),
		ProvenTxReqs:      make([]*TableProvenTxReq, 0),
		TxLabels:          make([]*TableTxLabel, 0),
		OutputTags:        make([]*TableOutputTag, 0),
		Transactions:      make([]*TableTransaction, 0),
		Outputs:           make([]*TableOutput, 0),
		TxLabelMaps:       make([]*TableTxLabelMap, 0),
		OutputTagMaps:     make([]*TableOutputTagMap, 0),
		Certificates:      make([]*TableCertificate, 0),
		CertificateFields: make([]*TableCertificateField, 0),
		Commissions:       make([]*TableCommission, 0),
	}
}

// FindOrInsertSyncStateAuthResponse represents the result of finding or inserting a sync state with authentication.
// It contains the sync state details and indicates whether a new sync state record was created or already existed.
type FindOrInsertSyncStateAuthResponse struct {
	SyncState *TableSyncState `json:"syncState"`
	IsNew     bool            `json:"isNew"`
}

// ProcessSyncChunkResult represents the result of processing a synchronization data chunk for a storage entity.
type ProcessSyncChunkResult struct {
	Done         bool       `json:"done"`
	MaxUpdatedAt *time.Time `json:"maxUpdated_at,omitempty"` // The maximum updated_at value seen for this entity over chunks received during this update cycle.
	Updates      int        `json:"updates"`                 // The number of updates made to the entity.
	Inserts      int        `json:"inserts"`                 // The number of new items inserted into the entity.
}
