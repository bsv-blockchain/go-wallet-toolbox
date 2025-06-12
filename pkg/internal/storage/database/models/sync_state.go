package models

import (
	"encoding/json"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
)

type SyncState struct {
	gorm.Model

	UserID             int    `gorm:"index"`
	StorageIdentityKey string `gorm:"type:varchar(130);not null;uniqueIndex"`
	StorageName        string `gorm:"type:varchar(128);not null"`
	Status             wdk.SyncStatus
	RefNum             string `gorm:"nul null;uniqueIndex"`
	SyncMap            json.RawMessage
	When               *time.Time
	Satoshis           *int64
}
