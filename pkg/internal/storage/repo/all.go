package repo

import (
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
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
	*TxNotes
	*UserUTXOs

	DB *gorm.DB
}

// NewSQLRepositories builds every repository over db, with retry the policy each of them
// applies to the transactions it opens. Pass dbretry.Retrying for the top-level connection,
// which owns its transactions, and dbretry.Disabled for a handle already bound to an outer
// transaction (see runInTransaction).
func NewSQLRepositories(db *gorm.DB, retry *dbretry.Policy) *Repositories {
	query := genquery.Use(db)
	repositories := &Repositories{
		DB:            db,
		Migrator:      NewMigrator(db),
		Settings:      NewSettings(db),
		OutputBaskets: NewOutputBaskets(db, query, retry),
		Certificates:  NewCertificates(db, query),
		UTXOs:         NewUTXOs(db, query, retry),
		Transactions:  NewTransactions(db, query, retry),
		Outputs:       NewOutputs(db, query, retry),
		KnownTx:       NewKnownTxRepo(db, query, retry),
		Sync:          NewSync(db, query),
		SyncState:     NewSyncState(db),
		KeyValue:      NewKeyValue(db),
		Commission:    NewCommission(db, query),
		TxNotes:       NewTxNotes(db, query),
		UserUTXOs:     NewUserUTXOs(db, query),
	}
	repositories.Users = NewUsers(db, query, repositories.Settings, repositories.OutputBaskets)

	return repositories
}
