package wdk

// SyncStatus represents the status result of a synchronization process, such as success, error, or unknown.
type SyncStatus string

// SyncStatus constants represent the possible states of a sync operation between a user and a storage system.
const (
	SyncStatusSuccess    SyncStatus = "success"    // The Last sync of this user from this storage was successful.
	SyncStatusError      SyncStatus = "error"      // The Last sync protocol operation for this user to this storage threw and error.
	SyncStatusIdentified SyncStatus = "identified" // sync storage has been identified but not sync'ed.
	SyncStatusUpdated    SyncStatus = "updated"    // TODO: In TS code, it's not documented, we need to clarify its meaning.
	SyncStatusUnknown    SyncStatus = "unknown"    // Sync protocol state is unknown.
)
