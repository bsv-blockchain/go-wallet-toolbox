package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// AdminCountsRow is a scan target for counts produced by AdminStats queries.
type AdminCountsRow struct {
	Day   int64 `gorm:"column:cnt_day"`
	Week  int64 `gorm:"column:cnt_week"`
	Month int64 `gorm:"column:cnt_month"`
	Total int64 `gorm:"column:cnt_total"`
}

func (r AdminCountsRow) counts() wdk.AdminStatsCounts {
	return wdk.AdminStatsCounts{Day: r.Day, Week: r.Week, Month: r.Month, Total: r.Total}
}

func addCounts(a, b wdk.AdminStatsCounts) wdk.AdminStatsCounts {
	return wdk.AdminStatsCounts{Day: a.Day + b.Day, Week: a.Week + b.Week, Month: a.Month + b.Month, Total: a.Total + b.Total}
}

// AdminStats aggregates storage-wide counters created after each of the given cutoffs and in total.
func (r *Repositories) AdminStats(ctx context.Context, dayAgo, weekAgo, monthAgo time.Time) (*wdk.AdminStats, error) {
	db := r.DB.WithContext(ctx)

	// countsSelect sums value over rows created after each cutoff and over all rows.
	countsSelect := func(value, createdAt string) (string, []any) {
		sum := func(alias string) string {
			return fmt.Sprintf("COALESCE(SUM(CASE WHEN %s > ? THEN %s ELSE 0 END), 0) AS %s", createdAt, value, alias)
		}
		return fmt.Sprintf("%s, %s, %s, COALESCE(SUM(%s), 0) AS cnt_total", sum("cnt_day"), sum("cnt_week"), sum("cnt_month"), value),
			[]any{dayAgo, weekAgo, monthAgo}
	}

	stats := &wdk.AdminStats{TxByStatus: make(map[wdk.TxStatus]wdk.AdminStatsCounts)}

	simple := []struct {
		model any
		dest  *wdk.AdminStatsCounts
	}{
		{&models.User{}, &stats.Users},
		{&models.OutputBasket{}, &stats.Baskets},
		{&models.Label{}, &stats.Labels},
		{&models.Tag{}, &stats.Tags},
	}
	for _, s := range simple {
		var row AdminCountsRow
		query, args := countsSelect("1", "created_at")
		if err := db.Model(s.model).Select(query, args...).Scan(&row).Error; err != nil {
			return nil, fmt.Errorf("failed to count %T: %w", s.model, err)
		}
		*s.dest = row.counts()
	}

	var txRows []struct {
		AdminCountsRow

		Status wdk.TxStatus
		NoTxID bool
	}
	query, args := countsSelect("1", "created_at")
	err := db.Model(&models.Transaction{}).
		Select("status, tx_id IS NULL AS no_tx_id, "+query, args...).
		Group("status, tx_id IS NULL").
		Scan(&txRows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count transactions: %w", err)
	}
	for _, row := range txRows {
		c := row.counts()
		stats.Transactions = addCounts(stats.Transactions, c)
		if row.Status == wdk.TxStatusFailed && row.NoTxID {
			stats.TxAbandoned = addCounts(stats.TxAbandoned, c)
			continue
		}
		stats.TxByStatus[row.Status] = addCounts(stats.TxByStatus[row.Status], c)
	}

	var satRows []struct {
		AdminCountsRow

		Change bool
	}
	outputs, transactions := r.DB.NamingStrategy.TableName("Output"), r.DB.NamingStrategy.TableName("Transaction")
	query, args = countsSelect(outputs+".satoshis", outputs+".created_at")
	err = db.Model(&models.Output{}).
		Select(outputs+".change, "+query, args...).
		Joins(fmt.Sprintf("JOIN %[1]s ON %[1]s.id = %[2]s.transaction_id AND %[1]s.deleted_at IS NULL", transactions, outputs)).
		Where(fmt.Sprintf("%[1]s.status = ? AND %[2]s.spendable = ?", transactions, outputs), wdk.TxStatusCompleted, true).
		Where(fmt.Sprintf("(%[1]s.change = ? OR %[1]s.basket_name IS NOT NULL)", outputs), true).
		Group(outputs + ".change").
		Scan(&satRows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to sum satoshis: %w", err)
	}
	for _, row := range satRows {
		if row.Change {
			stats.SatoshisDefault = row.counts()
		} else {
			stats.SatoshisOther = row.counts()
		}
	}

	return stats, nil
}
