package repo

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
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
}

func NewSync(db *gorm.DB, gen *genquery.Query) *Sync {
	return &Sync{
		db: db,

		SyncBasket:      syncrepo.NewSyncBasket(db, gen),
		SyncKnownTx:     syncrepo.NewSyncKnownTx(db, gen),
		SyncTransaction: syncrepo.NewSyncTransaction(db, gen),
		SyncOutput:      syncrepo.NewSyncOutput(db, gen),
		SyncLabel:       syncrepo.NewSyncLabel(db, gen),
		SyncLabelMap:    syncrepo.NewSyncLabelMap(db, gen),
		SyncTag:         syncrepo.NewSyncTag(db, gen),
		SyncTagMap:      syncrepo.NewSyncTagMap(db, gen),
	}
}
