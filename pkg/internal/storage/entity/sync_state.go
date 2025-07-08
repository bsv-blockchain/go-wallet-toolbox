package entity

import (
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
)

type SyncState struct {
	ID                 uint
	CreatedAt          time.Time
	UpdatedAt          time.Time
	UserID             int
	StorageIdentityKey string
	StorageName        string
	Status             wdk.SyncStatus
	Reference          string
	SyncMap            wdk.SyncMap
	When               *time.Time
	Satoshis           *satoshi.Value
}

func (ss *SyncState) ToWDK() (*wdk.TableSyncState, error) {
	syncMap, err := ss.SyncMap.JSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sync state: %w", err)
	}

	model := &wdk.TableSyncState{
		CreatedAt:          ss.CreatedAt,
		UpdatedAt:          ss.UpdatedAt,
		SyncStateID:        ss.ID,
		UserID:             ss.UserID,
		StorageIdentityKey: ss.StorageIdentityKey,
		StorageName:        ss.StorageName,
		Status:             ss.Status,
		Init:               false, // Init as true appears to be used only for testing purposes in TS version
		RefNum:             ss.Reference,
		SyncMap:            string(syncMap),
		When:               ss.When,
	}

	if ss.Satoshis != nil {
		model.Satoshis = to.Ptr(primitives.SatoshiValue(ss.Satoshis.MustUInt64()))
	}

	return model, nil
}
