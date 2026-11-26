package database

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

type optionsRow struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func (optionsRow) TableName() string { return "options_rows" }

func newOptionsDB(t *testing.T, mutate func(*defs.Database)) *gorm.DB {
	t.Helper()
	cfg := defs.DefaultDBConfig()
	// the DSN carries _txlock=immediate explicitly: this is how write-heavy deployments run SQLite
	cfg.SQLite.ConnectionString = "file:" + filepath.Join(t.TempDir(), "options.sqlite") + "?_txlock=immediate"
	cfg.MaxOpenConnections = 4
	cfg.MaxIdleConnections = 4
	mutate(&cfg)

	db, err := NewDatabase(cfg, logging.NewTestLogger(t))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.DB.AutoMigrate(&optionsRow{}))

	return db.DB
}

func TestGormConfig_OptionsAreOffByDefault(t *testing.T) {
	cfg := createGormConfig(&SlogGormLogger{}, defs.DefaultDBConfig())
	assert.False(t, cfg.PrepareStmt)
	assert.Zero(t, cfg.PrepareStmtMaxSize)
	assert.Zero(t, cfg.CreateBatchSize)
	assert.False(t, cfg.SkipDefaultTransaction, "SkipDefaultTransaction is not touched by these options")
}

func TestGormConfig_OptionsAreApplied(t *testing.T) {
	c := defs.DefaultDBConfig()
	c.PrepareStatements = true
	c.InsertBatchSize = 250

	cfg := createGormConfig(&SlogGormLogger{}, c)
	assert.True(t, cfg.PrepareStmt)
	assert.Equal(t, defs.DBPreparedStatementCacheSize, cfg.PrepareStmtMaxSize)
	assert.Equal(t, 250, cfg.CreateBatchSize)
}

func TestDatabaseValidate_RejectsNegativeInsertBatchSize(t *testing.T) {
	c := defs.DefaultDBConfig()
	c.InsertBatchSize = -1
	require.Error(t, c.Validate())
	c.InsertBatchSize = 0
	require.NoError(t, c.Validate())
}

// TestPrepareStatements_SQLite runs the situations that can break a cached prepared statement on
// SQLite: reuse inside transactions with commit and rollback under _txlock=immediate, concurrent
// writers, and a schema change after the statements were cached.
func TestPrepareStatements_SQLite(t *testing.T) {
	for _, prepare := range []bool{false, true} {
		t.Run(fmt.Sprintf("prepare_%v", prepare), func(t *testing.T) {
			db := newOptionsDB(t, func(c *defs.Database) { c.PrepareStatements = prepare })

			// same statement text reused across transactions, one committed and one rolled back
			for i := range 3 {
				require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
					return tx.Create(&optionsRow{Name: fmt.Sprintf("commit-%d", i)}).Error
				}))
			}
			errRollback := assert.AnError
			require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
				require.NoError(t, tx.Create(&optionsRow{Name: "rolled-back"}).Error)
				return errRollback
			}), errRollback)

			var count int64
			require.NoError(t, db.Model(&optionsRow{}).Count(&count).Error)
			assert.EqualValues(t, 3, count)

			// concurrent writers through immediate transactions
			var wg sync.WaitGroup
			errs := make(chan error, 80)
			for w := range 8 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := range 10 {
						errs <- db.Transaction(func(tx *gorm.DB) error {
							return tx.Create(&optionsRow{Name: fmt.Sprintf("w%d-%d", w, i)}).Error
						})
					}
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				require.NoError(t, err)
			}

			// schema change after the SELECT * statement was prepared and cached
			var rows []optionsRow
			require.NoError(t, db.Find(&rows).Error)
			require.NoError(t, db.Exec("ALTER TABLE options_rows ADD COLUMN extra text").Error)
			var after []map[string]any
			require.NoError(t, db.Raw("SELECT * FROM options_rows WHERE id = ?", rows[0].ID).Scan(&after).Error)
			require.Len(t, after, 1)
			assert.Contains(t, after[0], "extra", "a statement prepared before the ALTER sees the new column")
			require.NoError(t, db.Find(&rows).Error, "the cached gorm statement keeps working after the schema change")
		})
	}
}

// TestInsertBatchSize_SplitsSliceInserts checks that the option reaches GORM and bounds the rows
// per INSERT for slice creates.
func TestInsertBatchSize_SplitsSliceInserts(t *testing.T) {
	for size, wantStatements := range map[int]int{0: 1, 3: 4, 100: 1} {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			db := newOptionsDB(t, func(c *defs.Database) { c.InsertBatchSize = size })

			statements := 0
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("count", func(d *gorm.DB) {
				statements++
			}))

			rows := make([]*optionsRow, 10)
			for i := range rows {
				rows[i] = &optionsRow{Name: fmt.Sprintf("r%d", i)}
			}
			require.NoError(t, db.Create(&rows).Error)

			assert.Equal(t, wantStatements, statements)
			var count int64
			require.NoError(t, db.Model(&optionsRow{}).Count(&count).Error)
			assert.EqualValues(t, 10, count)
			for _, r := range rows {
				assert.NotZero(t, r.ID)
			}
		})
	}
}
