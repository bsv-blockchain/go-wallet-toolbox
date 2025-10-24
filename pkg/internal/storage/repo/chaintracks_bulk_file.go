package repo

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"gorm.io/gorm"
)

type ChaintracksBulkFile struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewCChaintracksBulkFile(db *gorm.DB, query *genquery.Query) *ChaintracksBulkFile {
	return &ChaintracksBulkFile{db: db, query: query}
}
