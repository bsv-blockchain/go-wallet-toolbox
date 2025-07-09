package wdk

import (
	"encoding/json"
	"fmt"
	"time"
)

// SyncMap is a map from EntityName to SyncMapEntity representing synchronization mapping state for multiple entities.
type SyncMap map[EntityName]*SyncMapEntity

// NewSyncMapFromJSON unmarshals JSON data into a SyncMap structure.
func NewSyncMapFromJSON(data []byte) (SyncMap, error) {
	var syncMap SyncMap
	if err := json.Unmarshal(data, &syncMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SyncMap: %w", err)
	}
	return syncMap, nil
}

// NewSyncMap creates and returns a new SyncMap initialized with entries for all entities in AllEntityNames.
func NewSyncMap() SyncMap {
	syncMap := make(SyncMap, len(AllEntityNames))
	for _, entityName := range AllEntityNames {
		syncMap[entityName] = NewSyncMapEntity(entityName)
	}
	return syncMap
}

// JSON serializes the SyncMap as JSON and returns the resulting byte slice or an error if marshaling fails.
func (sm SyncMap) JSON() ([]byte, error) {
	data, err := json.Marshal(sm)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SyncMap: %w", err)
	}
	return data, nil
}

// MaxUpdatedAt returns the latest non-nil MaxUpdatedAt timestamp among all entities in the SyncMap, or nil if none exist.
func (sm SyncMap) MaxUpdatedAt() *time.Time {
	var maxUpdatedAt *time.Time
	for _, entity := range sm {
		if entity.MaxUpdatedAt == nil {
			continue
		}
		if maxUpdatedAt == nil || entity.MaxUpdatedAt.After(*maxUpdatedAt) {
			maxUpdatedAt = entity.MaxUpdatedAt

		}
	}
	return maxUpdatedAt
}

// SyncMapEntity holds synchronization state for a specific entity, including id mapping, update time and item count.
type SyncMapEntity struct {
	// EntityName is the name of the entity in the sync map.
	EntityName EntityName `json:"entityName"`

	// IDMap maps foreign ids to local ids
	// NOTE: Some entities don't have idMaps (CertificateField, TxLabelMap and OutputTagMap)
	IDMap map[int]int `json:"idMap"`

	// MaxUpdatedAt - the maximum updated_at value seen for this entity over chunks received during this update cycle.
	MaxUpdatedAt *time.Time `json:"maxUpdated_at,omitempty"`

	// Count - the cumulative count of items of this entity type received over all the `SyncChunk`s since the `since` was last updated.
	// This is the `offset` value to use for the next SyncChunk request.
	Count uint64 `json:"count"`
}

// NewSyncMapEntity creates and returns a pointer to a new SyncMapEntity for the given entity name.
// The returned SyncMapEntity will have its EntityName set and an initialized empty IDMap.
func NewSyncMapEntity(entityName EntityName) *SyncMapEntity {
	return &SyncMapEntity{
		EntityName: entityName,
		IDMap:      make(map[int]int),
	}
}
