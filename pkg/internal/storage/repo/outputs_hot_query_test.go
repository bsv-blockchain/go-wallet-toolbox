package repo_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
)

func TestHotQueries_ReturnTargetChangeOutputsOnly(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	transactionID, txID := seedHotQueryOutputs(t, db.DB, 50)

	findChange := repo.NewUnspentChangeOutputsFinderForTest(db.DB)
	findSpendable := repo.NewSpendableChangeOutputsFinderForTest(db.DB)

	found, err := findChange(hotQueryUserID, transactionID)
	require.NoError(t, err)
	require.Len(t, found, 2)

	none, err := findChange(hotQueryUserID+1, transactionID)
	require.NoError(t, err)
	require.Empty(t, none)

	count, err := findSpendable(t.Context(), txID)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	count, err = findSpendable(t.Context(), "unknown")
	require.NoError(t, err)
	require.Zero(t, count)
}

// TestHotQueries_UseTransactionIDIndex asserts, on SQLite without ANALYZE, that both hot
// queries are planned from a transaction_id index instead of scanning the user_id/spendable index.
func TestHotQueries_UseTransactionIDIndex(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	if db.DB.Name() != "sqlite" {
		t.Skip("query plan assertions are SQLite specific")
	}

	transactionID, txID := seedHotQueryOutputs(t, db.DB, 200)

	var captured []string
	capture := db.DB.Session(&gorm.Session{
		Logger: logger.New(captureWriter{&captured}, logger.Config{LogLevel: logger.Info}),
	})

	_, err := repo.NewUnspentChangeOutputsFinderForTest(capture)(hotQueryUserID, transactionID)
	require.NoError(t, err)
	_, err = repo.NewSpendableChangeOutputsFinderForTest(capture)(t.Context(), txID)
	require.NoError(t, err)

	checked := 0
	for _, line := range captured {
		match := loggedSelect.FindStringSubmatch(line)
		if match == nil || !strings.Contains(match[1], "FROM `bsv_outputs`") {
			continue
		}
		sql := match[1]
		var plan []string
		rows, err := db.DB.Raw("EXPLAIN QUERY PLAN " + sql).Rows()
		require.NoError(t, err, sql)
		for rows.Next() {
			var id, parent, unused int
			var detail string
			require.NoError(t, rows.Scan(&id, &parent, &unused, &detail))
			plan = append(plan, detail)
		}
		require.NoError(t, rows.Close())

		joined := strings.Join(plan, "\n")
		require.Contains(t, joined, "bsv_outputs_transaction_id", "plan for %s:\n%s", sql, joined)
		require.NotContains(t, joined, "idx_bsv_outputs_spendable", "plan for %s:\n%s", sql, joined)
		require.NotContains(t, joined, "idx_bsv_outputs_user_id (", "plan for %s:\n%s", sql, joined)
		checked++
	}
	require.Equal(t, 2, checked)
}

// loggedSelect extracts the SQL from a gorm trace line ("<caller>\n[1.2ms] [rows:2] SELECT ...").
var loggedSelect = regexp.MustCompile(`(?s)\[rows:[^\]]+\] (SELECT .*)$`)

type captureWriter struct{ lines *[]string }

func (w captureWriter) Printf(format string, args ...any) {
	*w.lines = append(*w.lines, fmt.Sprintf(format, args...))
}
