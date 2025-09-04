package repo

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"gorm.io/gorm"
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
	*KnownTx
	*Sync
	*SyncState
	*KeyValue
	*Commission
}

func NewSQLRepositories(db *gorm.DB) *Repositories {
	query := genquery.Use(db)
	repositories := &Repositories{
		Migrator:      NewMigrator(db),
		Settings:      NewSettings(db),
		OutputBaskets: NewOutputBaskets(db),
		Certificates:  NewCertificates(db),
		UTXOs:         NewUTXOs(db, query),
		Transactions:  NewTransactions(db, query),
		Outputs:       NewOutputs(db, query),
		KnownTx:       NewKnownTxRepo(db, query),
		Sync:          NewSync(db, query),
		SyncState:     NewSyncState(db),
		KeyValue:      NewKeyValue(db),
		Commission:    NewCommission(db, query),
	}
	repositories.Users = NewUsers(db, query, repositories.Settings, repositories.OutputBaskets)

	return repositories
}
