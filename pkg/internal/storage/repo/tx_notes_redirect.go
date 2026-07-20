package repo

import (
	"encoding/json"
	"time"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"gorm.io/gorm"
)

// addTxNote appends a single history note to the corresponding bsv_proven_tx_reqs record
func addTxNote(tx *gorm.DB, note *pkgentity.TxHistoryNote) error {
	return addTxNotes(tx, []*pkgentity.TxHistoryNote{note})
}

// addTxNotes appends multiple history notes to the corresponding bsv_proven_tx_reqs record
func addTxNotes(tx *gorm.DB, notes []*pkgentity.TxHistoryNote) error {
	if len(notes) == 0 {
		return nil
	}

	// Group notes by TxID
	notesByTxID := make(map[string][]pkgentity.ReqHistoryNote)
	for _, n := range notes {
		// convert entity.TxHistoryNote to entity.ReqHistoryNote
		hn := make(pkgentity.ReqHistoryNote)
		hn["what"] = n.What
		if !n.When.IsZero() {
			hn["when"] = n.When.Format(time.RFC3339)
		}
		for k, v := range n.Attributes {
			hn[k] = v
		}
		if n.UserID != nil {
			hn["userId"] = *n.UserID
		}
		notesByTxID[n.TxID] = append(notesByTxID[n.TxID], hn)
	}

	for txID, reqNotes := range notesByTxID {
		// Load the history blob for this txID
		var req models.ProvenTxReq
		err := tx.Select("history").Where("txid = ?", txID).First(&req).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// No proven tx req found, nothing to do
				continue
			}
			return err
		}

		var history pkgentity.ProvenTxReqHistory
		if req.History != "" {
			_ = json.Unmarshal([]byte(req.History), &history)
		}

		for _, rn := range reqNotes {
			history.AddHistoryNote(rn, true) // dedup
		}

		newBlob, err := json.Marshal(history)
		if err != nil {
			return err
		}

		err = tx.Model(&models.ProvenTxReq{}).Where("txid = ?", txID).Update("history", string(newBlob)).Error
		if err != nil {
			return err
		}
	}
	return nil
}
