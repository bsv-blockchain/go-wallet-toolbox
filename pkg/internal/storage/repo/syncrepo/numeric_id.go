package syncrepo

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// upsertNumericIDLookup inserts string IDs into the numeric ID lookup table to ensure each string ID has a corresponding numeric ID.
// It executes custom INSERT ... SELECT ... ON CONFLICT DO NOTHING based on the result of the provided stringIDsQuery function.
func upsertNumericIDLookup(ctx context.Context, db, tx *gorm.DB, query *genquery.Query, stringIDsQuery func(db *gorm.DB) *gorm.DB) error {
	dry := db.Session(&gorm.Session{DryRun: true, Initialized: true}) // NOTICE: Initialized to separate the dry run from the actual transaction (this makes the Session to clone the Statement)
	queryForStringID := stringIDsQuery(dry)

	insertSelect := &gorm.Statement{DB: db}
	clause.Expr{
		SQL:  fmt.Sprintf("INSERT INTO %s (table_name, string_id) %s ON CONFLICT DO NOTHING", query.NumericIDLookup.TableName(), queryForStringID.Statement.SQL.String()),
		Vars: queryForStringID.Statement.Vars,
	}.Build(insertSelect)

	err := tx.WithContext(ctx).Exec(insertSelect.SQL.String(), insertSelect.Vars...).Error
	if err != nil {
		return fmt.Errorf("failed to create numeric ID lookup rows: %w", err)
	}

	return nil
}

// joinWithNumericIDLookupScope returns a GORM scope to join a numeric ID lookup table based on the provided string ID clause.
// The entityName is used to specify the table_name of the entity, and the stringIDClause is used to match the string_id in the numeric ID lookup table.
func joinWithNumericIDLookupScope(query *genquery.Query, stringIDClause string, entityName string, join clause.JoinType) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		joinQuery := fmt.Sprintf("%s JOIN %s as num on num.table_name = ? and num.string_id = %s", join, query.NumericIDLookup.TableName(), stringIDClause)

		return db.Joins(joinQuery, entityName)
	}
}

func findNumericIDLookup(ctx context.Context, tx *gorm.DB, tableName string, stringID string) (uint, error) {
	var numericID uint
	txScan := tx.WithContext(ctx).
		Model(&models.NumericIDLookup{}).
		Select("num_id").
		Where("table_name = ? AND string_id = ?", tableName, stringID).
		Scan(&numericID)
	if txScan.Error != nil {
		return 0, fmt.Errorf("failed to find numeric ID for %q: %w", stringID, txScan.Error)
	}

	if txScan.RowsAffected == 0 {
		return 0, fmt.Errorf("numeric ID not found for %q", stringID)
	}

	return numericID, nil
}

func saveNumericIDLookup(ctx context.Context, tx *gorm.DB, tableName string, stringID string) error {
	stringIDLookup := &models.NumericIDLookup{
		TableName: tableName,
		StringID:  stringID,
	}

	err := tx.
		WithContext(ctx).
		Clauses(clause.OnConflict{
			DoNothing: true,
		}).
		Create(stringIDLookup).Error
	if err != nil {
		return fmt.Errorf("failed to save numeric ID lookup for %q: %w", stringID, err)
	}

	return nil
}
