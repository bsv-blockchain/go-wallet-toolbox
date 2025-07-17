package builders

import (
	"fmt"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
)

type OutpointsQuery struct {
	Outpoints []wdk.OutPoint
	Session   *gorm.DB
}

func (b *OutpointsQuery) CreateQuery() *gorm.DB {
	values := make([]any, 0, len(b.Outpoints))
	placeholders := make([]string, 0, len(b.Outpoints))

	for _, outpoint := range b.Outpoints {
		placeholders = append(placeholders, "(?, ?)")
		values = append(values, outpoint.TxID, outpoint.Vout)
	}

	return b.Session.Where(fmt.Sprintf("(transaction_id, vout) IN (%s)", strings.Join(placeholders, ", ")), values...)
}
