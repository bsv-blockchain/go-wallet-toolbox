package repo

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"gorm.io/gorm"
)

type naming struct {
	outputBasketTableName    string
	numericIDLookupTableName string
	provenTxReqTableName     string
	transactionsTableName    string
}

func newNaming(db *gorm.DB) *naming {

	return &naming{
		outputBasketTableName:    getTableName(db, &models.OutputBasket{}),
		numericIDLookupTableName: getTableName(db, &models.NumericIDLookup{}),
		provenTxReqTableName:     getTableName(db, &models.ProvenTxReq{}),
		transactionsTableName:    getTableName(db, &models.Transaction{}),
	}
}

func getTableName(db *gorm.DB, model any) string {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		panic(err) // This should not happen, as we are using a valid model
	}
	return stmt.Table
}
