package repo

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"gorm.io/gorm"
)

type naming struct {
	outputBasketTableName    string
	numericIDLookupTableName string
	knownTxTableName         string
	transactionsTableName    string
	outputsTableName         string
	labelsTableName          string
	labelsMapTableName       string
}

func newNaming(db *gorm.DB) *naming {

	return &naming{
		outputBasketTableName:    getTableName(db, &models.OutputBasket{}),
		numericIDLookupTableName: getTableName(db, &models.NumericIDLookup{}),
		knownTxTableName:         getTableName(db, &models.KnownTx{}),
		transactionsTableName:    getTableName(db, &models.Transaction{}),
		outputsTableName:         getTableName(db, &models.Output{}),
		labelsTableName:          getTableName(db, &models.Label{}),
		labelsMapTableName:       getTableName(db, &models.TransactionLabel{}),
	}
}

func getTableName(db *gorm.DB, model any) string {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		panic(err) // This should not happen, as we are using a valid model
	}
	return stmt.Table
}
