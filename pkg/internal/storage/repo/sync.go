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

func NewSync(db *gorm.DB, query *genquery.Query) *Sync {
	return &Sync{
		db: db,

		SyncBasket:      syncrepo.NewSyncBasket(db, query),
		SyncKnownTx:     syncrepo.NewSyncKnownTx(db, query),
		SyncTransaction: syncrepo.NewSyncTransaction(db, query),
		SyncOutput:      syncrepo.NewSyncOutput(db, query),
		SyncLabel:       syncrepo.NewSyncLabel(db, query),
		SyncLabelMap:    syncrepo.NewSyncLabelMap(db, query),
		SyncTag:         syncrepo.NewSyncTag(db, query),
		SyncTagMap:      syncrepo.NewSyncTagMap(db, query),
	}
}
