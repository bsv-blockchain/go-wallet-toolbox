package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
)

// NewUnspentChangeOutputsFinderForTest exposes the change-output lookup of SpendTransaction to
// the external test package. The query builder is created once, outside the returned function.
func NewUnspentChangeOutputsFinderForTest(tx *gorm.DB) func(userID int, transactionID uint) ([]*models.Output, error) {
	query := genquery.Use(tx)
	return func(userID int, transactionID uint) ([]*models.Output, error) {
		return findUnspentChangeOutputs(tx, query, userID, transactionID)
	}
}

// NewSpendableChangeOutputsFinderForTest exposes the change-output lookup of
// createUTXOsForSpendableOutputsByTxID to the external test package. The returned function
// reports how many outputs were found.
func NewSpendableChangeOutputsFinderForTest(tx *gorm.DB) func(ctx context.Context, txID string) (int, error) {
	query := genquery.Use(tx)
	return func(ctx context.Context, txID string) (int, error) {
		outputs, err := findSpendableChangeOutputsByTxID(ctx, query, txID)
		return len(outputs), err
	}
}
