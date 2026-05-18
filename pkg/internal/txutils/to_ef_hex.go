package txutils

import (
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// ValidateSingleLeafTx checks that BEEF contains exactly one leaf transaction.
// A leaf is a transaction (mined or unmined) whose txid does not appear in any
// of the inputs (as SourceTXID) of any transaction within the BEEF.
// Returns error if there are zero or multiple leaves (e.g. disconnected chains
// or multiple independent subject transactions).
// Call BindBumpsAndTransactions once before calling this.
func ValidateSingleLeafTx(beef *transaction.Beef) error {
	if beef == nil || len(beef.Transactions) == 0 {
		return fmt.Errorf("expected only one subject tx, but got 0")
	}

	referenced := referencedParentTxIDs(beef)

	leafCount := 0
	for txid := range beef.Transactions {
		if _, isReferenced := referenced[txid]; !isReferenced {
			leafCount++
		}
	}

	if leafCount != 1 {
		return fmt.Errorf("expected only one subject tx, but got %d", leafCount)
	}
	return nil
}

// referencedParentTxIDs returns the set of txids inside the BEEF that are
// used as a parent (i.e. appear as SourceTXID in some input of another tx
// that is also inside the BEEF). We include inputs from mined transactions
// so that internal mined ancestors do not falsely appear as additional leaves.
func referencedParentTxIDs(beef *transaction.Beef) map[chainhash.Hash]struct{} {
	referenced := make(map[chainhash.Hash]struct{})
	for _, btx := range beef.Transactions {
		if btx.Transaction == nil {
			continue
		}
		for _, input := range btx.Transaction.Inputs {
			if input.SourceTXID == nil {
				continue
			}
			if _, ok := beef.Transactions[*input.SourceTXID]; ok {
				referenced[*input.SourceTXID] = struct{}{}
			}
		}
	}
	return referenced
}

// BindBumpsAndTransactions binds BUMPs to transactions in BEEF.
func BindBumpsAndTransactions(beef *transaction.Beef, logger *slog.Logger) {
	for i, bump := range beef.BUMPs {
		if len(bump.Path) == 0 || len(bump.Path[0]) == 0 {
			logger.Warn("got bump without bottom path", slog.String("merklePath", bump.Hex()))
			continue
		}
		for _, element := range bump.Path[0] {
			if element.Txid != nil && *element.Txid {
				if element.Hash == nil {
					logger.Error("got leaf marked as txid in BUMP but hash is nil")
					continue
				}
				tx, ok := beef.Transactions[*element.Hash]
				if !ok {
					logger.Warn("got leaf marked as txid in BUMP that is not part of the BEEF", slog.String("txid", element.Hash.String()))
					continue
				}
				tx.BumpIndex = i
				tx.DataFormat = transaction.RawTxAndBumpIndex
				tx.Transaction.MerklePath = bump
			}
		}
	}
}
