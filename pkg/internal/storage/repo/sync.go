package repo

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo/syncrepo"
	"gorm.io/gorm"
)

type Sync struct {
	*syncrepo.SyncBasket
	*syncrepo.SyncKnownTx
	*syncrepo.SyncTransaction
	*syncrepo.SyncOutput
	*syncrepo.SyncLabel
	*syncrepo.SyncLabelMap
	*syncrepo.SyncTag
	*syncrepo.SyncTagMap
	db *gorm.DB

	naming *naming
}

func NewSync(db *gorm.DB) *Sync {
	return &Sync{
		db:     db,
		naming: newNaming(db),

		SyncBasket:      syncrepo.NewSyncBasket(db),
		SyncKnownTx:     syncrepo.NewSyncKnownTx(db),
		SyncTransaction: syncrepo.NewSyncTransaction(db),
		SyncOutput:      syncrepo.NewSyncOutput(db),
		SyncLabel:       syncrepo.NewSyncLabel(db),
		SyncLabelMap:    syncrepo.NewSyncLabelMap(db),
		SyncTag:         syncrepo.NewSyncTag(db),
		SyncTagMap:      syncrepo.NewSyncTagMap(db),
	}
}
