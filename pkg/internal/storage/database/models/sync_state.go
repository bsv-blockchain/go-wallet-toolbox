package models

import (
	"encoding/json"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type SyncState struct {
	Timestamps

	SyncStateID        uint            `gorm:"column:syncStateId;primaryKey;autoIncrement"`
	UserID             int             `gorm:"column:userId;uniqueIndex:idx_user_storage_key"`
	StorageIdentityKey string          `gorm:"column:storageIdentityKey;type:varchar(130);not null;default:'';uniqueIndex:idx_user_storage_key"`
	StorageName        string          `gorm:"column:storageName;type:varchar(128);not null;default:''"`
	Status             wdk.SyncStatus  `gorm:"column:status;default:'unknown'"`
	RefNum             string          `gorm:"column:refNum;not null;default:'';uniqueIndex"`
	SyncMap            json.RawMessage `gorm:"column:syncMap"`
	When               *time.Time      `gorm:"column:when"`
	Satoshis           *int64          `gorm:"column:satoshis"`
	Init               bool            `gorm:"column:init;not null;default:false"`
	ErrorLocal         *string         `gorm:"column:errorLocal;type:text"`
	ErrorOther         *string         `gorm:"column:errorOther;type:text"`
}
