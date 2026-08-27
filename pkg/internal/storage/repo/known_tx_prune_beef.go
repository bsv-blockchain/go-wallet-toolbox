package repo

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
)

// pruneInputBeefIfReconstructible drops a transaction's stored input beef once
// every transaction that blob carries is also readable as a known tx row.
//
// The blob is the entire ancestry a transaction was submitted with. For a wallet
// spending its own change it duplicates rows that already exist in this table -
// and it duplicates them once per descendant, so a chain of self-funded spends
// stores the same ancestors over and over and the per-row cost grows with the
// depth of the chain. Measured on a 10k-transaction run: 1.67 GB of input beef
// against 5.5 MB of the raw_tx and merkle_path it was duplicating.
//
// The guard is deliberately "everything this blob provides is also a row", not
// "the transaction is proven" and not "its direct parents are rows":
//
//   - Inputs supplied by a caller through createAction's inputBEEF are recorded
//     ONLY inside this blob, never as known txs (see needsInputBEEF), so proof
//     status says nothing about whether the ancestry survives the delete.
//   - GetBEEF's MinProofLevel option walks PAST a proof, to a caller-chosen depth
//     (see directParentIDs). Checking only the direct parents would be sound at
//     depth 1 and unsound below it, because the blob also carries grandparents
//     that may exist nowhere else. Comparing against the blob's full contents is
//     what makes the invariant hold at any depth.
//
// A proof carried by the blob counts as content too: an ancestor whose row has no
// merkle_path is not fully covered by that row, so a blob proving it is kept.
//
// Pruning is skipped, never failed, when the row or the blob cannot be read: this
// is a space optimisation and must not turn a successful proof into an error.
func pruneInputBeefIfReconstructible(tx *gorm.DB, txID string) error {
	var row models.KnownTx
	err := tx.Model(&models.KnownTx{}).
		Select("input_beef").
		Where("tx_id = ?", txID).
		First(&row).Error
	if err != nil {
		// A missing row is not this function's problem to report.
		return nil //nolint:nilerr // pruning is best-effort by design
	}

	if len(row.InputBeef) == 0 {
		return nil
	}

	beef, err := transaction.NewBeefFromBytes(row.InputBeef)
	if err != nil {
		// An unparsable blob is left exactly as it is: it is still the only copy
		// of whatever it holds, and rewriting it is not this function's job.
		return nil //nolint:nilerr // pruning is best-effort by design
	}

	if !rowsCoverBeef(tx, beef) {
		return nil
	}

	err = tx.Model(&models.KnownTx{}).
		Where("tx_id = ?", txID).
		UpdateColumn("input_beef", nil).Error
	if err != nil {
		return fmt.Errorf("failed to prune input beef of known tx %s: %w", txID, err)
	}

	return nil
}

// rowsCoverBeef reports whether every transaction the beef carries is also
// available as a known tx row - the raw tx, and a merkle path wherever the beef
// proves one.
//
// TxIDOnly entries carry neither, so they are nothing to lose and are skipped.
func rowsCoverBeef(tx *gorm.DB, beef *transaction.Beef) bool {
	needed := make(map[string]bool, len(beef.Transactions)) // txid -> beef proves it
	for txid, beefTx := range beef.Transactions {
		if beefTx.DataFormat == transaction.TxIDOnly {
			continue
		}
		needed[txid.String()] = beefTx.DataFormat == transaction.RawTxAndBumpIndex
	}

	if len(needed) == 0 {
		return true
	}

	txIDs := make([]string, 0, len(needed))
	for txid := range needed {
		txIDs = append(txIDs, txid)
	}

	var rows []models.KnownTx
	if err := tx.Model(&models.KnownTx{}).
		Select("tx_id, raw_tx, merkle_path").
		Where("tx_id IN ?", txIDs).
		Find(&rows).Error; err != nil {
		return false
	}

	covered := 0
	for i := range rows {
		provenByBeef, wanted := needed[rows[i].TxID]
		if !wanted || len(rows[i].RawTx) == 0 {
			continue
		}
		if provenByBeef && !rows[i].HasMerklePath() {
			// The blob is the only place this ancestor's proof exists.
			return false
		}
		covered++
	}

	return covered == len(needed)
}
