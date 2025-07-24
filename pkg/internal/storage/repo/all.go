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
	gen := genquery.Use(db)
	repositories := &Repositories{
		Migrator:      NewMigrator(db),
		Settings:      NewSettings(db),
		OutputBaskets: NewOutputBaskets(db),
		Certificates:  NewCertificates(db),
		UTXOs:         NewUTXOs(db),
		Transactions:  NewTransactions(db),
		Outputs:       NewOutputs(db),
		KnownTx:       NewKnownTxRepo(db, gen),
		Sync:          NewSync(db, gen),
		SyncState:     NewSyncState(db),
		KeyValue:      NewKeyValue(db),
		Commission:    NewCommission(db, gen),
	}
	repositories.Users = NewUsers(db, repositories.Settings, repositories.OutputBaskets)

	return repositories
}
