package models

import (
	"encoding/json"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
)

type SyncState struct {
	gorm.Model

	UserID             int    `gorm:"uniqueIndex:idx_user_storage_device"`
	StorageIdentityKey string `gorm:"type:varchar(130);not null;uniqueIndex:idx_user_storage_device"`
	DeviceID           string `gorm:"type:varchar(64);not null;default:'';uniqueIndex:idx_user_storage_device"`
	StorageName        string `gorm:"type:varchar(128);not null"`
	Status             wdk.SyncStatus
	RefNum             string `gorm:"not null;uniqueIndex"`
	SyncMap            json.RawMessage
	When               *time.Time
	Satoshis           *int64
}
