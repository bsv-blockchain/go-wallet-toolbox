package repo

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"gorm.io/gorm"
)

type ChaintracksLiveHeader struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewChaintracksLiveHeader(db *gorm.DB, query *genquery.Query) *ChaintracksLiveHeader {
	return &ChaintracksLiveHeader{db: db, query: query}
}

