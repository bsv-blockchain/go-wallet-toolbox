package repo

import (
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
)

type Repositories struct {
	*Migrator
	*Settings
	*Users
	*OutputBaskets
	*Certificates
	*UTXOs
	*Transactions
	*Outputs
	*ProvenTxReqRepo
	*Sync
	*SyncState
	*KeyValue
	*Commission

	DB *gorm.DB
}

func NewSQLRepositories(db *gorm.DB) *Repositories {
	query := genquery.Use(db)
	repositories := &Repositories{
		DB:              db,
		Migrator:        NewMigrator(db),
		Settings:        NewSettings(db),
		OutputBaskets:   NewOutputBaskets(db, query),
		Certificates:    NewCertificates(db, query),
		UTXOs:           NewUTXOs(db, query),
		Transactions:    NewTransactions(db, query),
		Outputs:         NewOutputs(db, query),
		ProvenTxReqRepo: NewProvenTxReqRepo(db, query),
		Sync:            NewSync(db, query),
		SyncState:       NewSyncState(db),
		KeyValue:        NewKeyValue(db),
		Commission:      NewCommission(db, query),
	}
	repositories.Users = NewUsers(db, query, repositories.Settings, repositories.OutputBaskets)

	return repositories
}
