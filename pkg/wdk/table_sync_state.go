package wdk

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"time"
)

// TableSyncState represents the current synchronization state of a database table for a given user and storage.
type TableSyncState struct {
	CreatedAt          time.Time                `json:"created_at"`
	UpdatedAt          time.Time                `json:"updated_at"`
	SyncStateID        int                      `json:"syncStateId"`
	UserID             int                      `json:"userId"`
	StorageIdentityKey string                   `json:"storageIdentityKey"`
	StorageName        string                   `json:"storageName"`
	Status             SyncStatus               `json:"status"`
	Init               bool                     `json:"init"`
	RefNum             string                   `json:"refNum"`
	SyncMap            string                   `json:"syncMap"`
	When               *time.Time               `json:"when,omitempty"`
	Satoshis           *primitives.SatoshiValue `json:"satoshis,omitempty"`
	ErrorLocal         *string                  `json:"errorLocal,omitempty"`
	ErrorOther         *string                  `json:"errorOther,omitempty"`
}
