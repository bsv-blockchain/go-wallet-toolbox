package repo

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// RegisterAncestors records the transactions carried inside a caller-supplied
// inputBEEF as known tx rows.
//
// Without this, those transactions exist nowhere but the blob, which is why the
// blob has to be stored per descendant and why storage grows with the square of
// a chain's length: every spend duplicates the whole ancestry again. Recorded as
// rows, each ancestor is stored once and the ancestry walk reads it directly -
// the same shape persistNewProven already uses when GetBEEF fetches an unknown
// proven ancestor from a service.
//
// Writes are strictly additive. An ancestor that is already a known tx is left
// alone apart from filling genuinely empty columns: this runs concurrently with
// every other path that writes known txs, and an ancestor arriving inside a BEEF
// says nothing about a status that path may have established. Specifically it
// never changes an existing row's status, never clears a raw_tx, and never
// clears a merkle_path.
//
// Insertion uses ON CONFLICT DO NOTHING rather than read-then-write, so two
// callers submitting the same ancestor at once cannot race each other.
func (p *KnownTx) RegisterAncestors(ctx context.Context, reference string, ancestors []entity.AncestorTx) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-RegisterAncestors",
		attribute.String("Reference", reference),
		attribute.Int("Count", len(ancestors)),
	)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(ancestors) == 0 {
		return nil
	}

	err = runInTransaction(ctx, p.db, p.retry, func(tx *gorm.DB) error {
		for _, ancestor := range ancestors {
			if err := registerAncestor(tx, reference, ancestor); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to register ancestors for %s: %w", reference, err)
	}
	return nil
}

func registerAncestor(tx *gorm.DB, reference string, ancestor entity.AncestorTx) error {
	if ancestor.TxID == "" || len(ancestor.RawTx) == 0 {
		// Nothing worth a row: the walk needs a raw tx to reach this ancestor's
		// own parents, and a txid alone provides none.
		return nil
	}

	// A proof carried by the BEEF is evidence the transaction is mined. Without
	// one, "unmined" is the honest description: a caller is spending this output,
	// so the transaction has been sent, it simply has no proof yet.
	//
	// It must NOT be recorded as "unknown". That status is in
	// ProvenTxReqProblematicStatuses, which every BEEF build filters out, so an
	// ancestor recorded that way is written but unreachable - the ancestry walk
	// then fails to find it and the whole build errors.
	status := wdk.ProvenTxStatusUnmined
	if ancestor.IsProven() {
		status = wdk.ProvenTxStatusCompleted
	}

	row := models.KnownTx{
		TxID:       ancestor.TxID,
		Status:     status,
		RawTx:      ancestor.RawTx,
		MerklePath: ancestor.MerklePath,
		Notify:     "{}",
	}

	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return fmt.Errorf("failed to insert ancestor known tx %s: %w", ancestor.TxID, result.Error)
	}

	if result.RowsAffected > 0 {
		note := history.NewBuilder().AncestorFromInputBeef(reference).Entity(ancestor.TxID)
		if err := addTxNotes(tx, []*pkgentity.TxHistoryNote{note}); err != nil {
			return fmt.Errorf("failed to add ancestor history note for %s: %w", ancestor.TxID, err)
		}
		return nil
	}

	// The row already existed. Fill only what is genuinely absent - never
	// overwrite, never downgrade.
	return fillAncestorGaps(tx, ancestor)
}

func fillAncestorGaps(tx *gorm.DB, ancestor entity.AncestorTx) error {
	err := tx.Model(&models.KnownTx{}).
		Where("tx_id = ?", ancestor.TxID).
		Where("raw_tx IS NULL OR LENGTH(raw_tx) = 0").
		UpdateColumn("raw_tx", ancestor.RawTx).Error
	if err != nil {
		return fmt.Errorf("failed to fill raw tx of ancestor %s: %w", ancestor.TxID, err)
	}

	if !ancestor.IsProven() {
		return nil
	}

	// A proof is only ever added, never replaced: a row that already carries one
	// was proven by a path that also recorded the block it was proven in, and
	// this BEEF carries no such context.
	err = tx.Model(&models.KnownTx{}).
		Where("tx_id = ?", ancestor.TxID).
		Where("merkle_path IS NULL OR LENGTH(merkle_path) = 0").
		UpdateColumn("merkle_path", ancestor.MerklePath).Error
	if err != nil {
		return fmt.Errorf("failed to fill merkle path of ancestor %s: %w", ancestor.TxID, err)
	}

	return nil
}
