package models

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// UserUTXO is a table holding user's Unspent Transaction Outputs (UTXOs).
//
// idx_user_utxos_selection is a composite index over (user_id, basket_name,
// reserved_by_id, utxo_status, satoshis) supporting UTXO selection queries that
// filter/sort on this exact column set. ReservedByID also carries a plain,
// single-column index so lookups by reservation alone don't need the composite.
//
// idx_user_utxos_selection_order appends output_id as a sixth column so the
// bounded funder's `ORDER BY satoshis, output_id` micro-queries are pure index
// walks. This matters most for the throughput strategy's fuel pool, where ALL
// rows share one satoshis value and the five-column index would degenerate to
// sorting the whole equal-value range per claim. It supersedes
// idx_user_utxos_selection (a strict prefix), which is kept for existing
// deployments and can be dropped in a follow-up migration.
//
// idx_user_utxos_claim is a PARTIAL index (WHERE reserved_by_id IS NULL) over
// (user_id, basket_name, utxo_status, satoshis, output_id) — the funder's
// `... AND reserved_by_id IS NULL ORDER BY satoshis, output_id LIMIT 1 FOR
// UPDATE SKIP LOCKED` claim query. With reserved_by_id embedded at position 3
// of the full composite (idx_user_utxos_selection_order), the Postgres planner
// was observed choosing a sequential scan + external sort over a 250k-row fuel
// pool instead (~130 ms per claim — the dominant per-createAction latency at
// high TPS); the partial index makes the claim a pure index walk (<0.1 ms).
type UserUTXO struct {
	UserID   int     `gorm:"primaryKey;index:idx_user_utxos_selection,priority:1;index:idx_user_utxos_selection_order,priority:1;index:idx_user_utxos_claim,priority:1,where:reserved_by_id IS NULL"`
	OutputID uint    `gorm:"primaryKey;index:idx_user_utxos_selection_order,priority:6;index:idx_user_utxos_claim,priority:5"`
	Output   *Output `gorm:"foreignKey:OutputID"`

	UTXOStatus wdk.UTXOStatus `gorm:"index:idx_utxo_status;index:idx_user_utxos_selection,priority:4;index:idx_user_utxos_selection_order,priority:4;index:idx_user_utxos_claim,priority:3"`

	BasketName string        `gorm:"not null;index;index:idx_user_utxos_selection,priority:2;index:idx_user_utxos_selection_order,priority:2;index:idx_user_utxos_claim,priority:2"`
	Basket     *OutputBasket `gorm:"foreignKey:UserID,BasketName;references:UserID,Name"`

	Satoshis uint64 `gorm:"index:idx_user_utxos_selection,priority:5;index:idx_user_utxos_selection_order,priority:5;index:idx_user_utxos_claim,priority:4"`
	// EstimatedInputSize is the estimated size increase when adding and unlocking this UTXO to a transaction.
	EstimatedInputSize uint64
	CreatedAt          time.Time

	ReservedByID *uint `gorm:"index;index:idx_user_utxos_selection,priority:3;index:idx_user_utxos_selection_order,priority:3"`
	ReservedBy   *Transaction
}
