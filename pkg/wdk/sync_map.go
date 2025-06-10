package wdk

import (
	"encoding/json"
	"fmt"
	"time"
)

type SyncMap map[EntityName]SyncMapEntity

func NewSyncMapFromJSON(data []byte) (SyncMap, error) {
	var syncMap SyncMap
	if err := json.Unmarshal(data, &syncMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SyncMap: %w", err)
	}
	return syncMap, nil
}

func NewSyncMap() SyncMap {
	syncMap := make(SyncMap, len(AllEntityNames))
	for _, entityName := range AllEntityNames {
		syncMap[entityName] = SyncMapEntity{
			EntityName: entityName,
			IDMap:      make(map[int]int),
		}
	}
	return syncMap
}

func (sm SyncMap) JSON() ([]byte, error) {
	data, err := json.Marshal(sm)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SyncMap: %w", err)
	}
	return data, nil
}

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
