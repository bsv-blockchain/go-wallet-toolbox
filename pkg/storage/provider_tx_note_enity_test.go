package storage_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/crud"
)

func TestTxNoteFilters(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	provider := given.Provider()
	db := provider.GORM().Database.DB

	now := time.Now().Truncate(time.Second)
	seedTestTxNotes(t, db, now)

	tests := map[string]struct {
		filter func(r crud.TxNoteReader)
		expect int64
	}{
		"CreatedAt greater than now": {
			filter: func(r crud.TxNoteReader) {
				r.CreatedAt().GreaterThan(now)
			},
			expect: 1,
		},
		"CreatedAt less than now + 1m": {
			filter: func(r crud.TxNoteReader) {
				r.CreatedAt().LessThan(now.Add(1 * time.Minute))
			},
			expect: 4,
		},
		"CreatedAt between -90min and -15min": {
			filter: func(r crud.TxNoteReader) {
				r.CreatedAt().Between(now.Add(-90*time.Minute), now.Add(-15*time.Minute))
			},
			expect: 1,
		},
		"filter by What equals foo": {
			filter: func(r crud.TxNoteReader) {
				r.What().Equals("foo")
			},
			expect: 3,
		},
		"filter by UserID equals 2": {
			filter: func(r crud.TxNoteReader) {
				r.UserID().Equals(2)
			},
			expect: 2,
		},
		"filter by TxID equals B2": {
			filter: func(r crud.TxNoteReader) {
				r.TxID("B2")
			},
			expect: 1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// when:
			reader := provider.GORM().TxNoteEntity().Read()
			tc.filter(reader)

			// then:
			count, err := reader.Count(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tc.expect, count, "filter %s", name)
		})
	}
}

func insertTxNote(db *gorm.DB, txID, what string, userID *int, createdAt time.Time) (*models.TxNote, error) {
	note := &models.TxNote{
		TxID:      txID,
		What:      what,
		UserID:    userID,
		CreatedAt: createdAt,
	}
	if err := db.Create(note).Error; err != nil {
		return nil, err
	}
	return note, nil
}

func seedTestTxNotes(t *testing.T, db *gorm.DB, now time.Time) {
	t.Helper()

	u2 := 2

	notes := []struct {
		txID      string
		what      string
		userID    *int
		createdAt time.Time
	}{
		{"tx1", "foo", nil, now.Add(-2 * time.Hour)},
		{"tx2", "bar", nil, now.Add(1 * time.Minute)},
		{"tx3", "foo", nil, now.Add(-45 * time.Minute)},
		{"B2", "bar", &u2, now},
		{"C3", "foo", &u2, now},
	}

	for _, n := range notes {
		_, err := insertTxNote(db, n.txID, n.what, n.userID, n.createdAt)
		require.NoError(t, err)
	}
}
