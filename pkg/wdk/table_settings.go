package wdk

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// TableSettings is a struct that holds the settings of the whole DB
type TableSettings struct {
	StorageIdentityKey string          `json:"storageIdentityKey"`
	StorageName        string          `json:"storageName"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	Chain              defs.BSVNetwork `json:"chain"`
	DbType             defs.DBType     `json:"dbtype"`
	MaxOutputScript    int             `json:"maxOutputScript"`
}
