package txutils

import (
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// ParentsFirst returns txIDs reordered so that a transaction spending another
// transaction of the same batch comes after it, preserving the input order
// otherwise. PostFromBEEF posts a batch in slice order and Arcade forwards
// upstream in receive order, so an unordered batch could post a child before the
// parent it shares a request with. The input slice is not modified.
//
// The reordering is stable: transactions with no dependency on each other keep
// their relative order, so an input already sorted by creation time stays sorted.
func ParentsFirst(beef *transaction.Beef, txIDs []string) []string {
	if beef == nil || len(txIDs) < 2 {
		return append([]string(nil), txIDs...)
	}

	batch := make(map[string]struct{}, len(txIDs))
	for _, txID := range txIDs {
		batch[txID] = struct{}{}
	}

	ordered := make([]string, 0, len(txIDs))
	visited := make(map[string]struct{}, len(txIDs))

	var visit func(txID string)
	visit = func(txID string) {
		if _, seen := visited[txID]; seen {
			return
		}
		visited[txID] = struct{}{} // marked before recursing, so a cycle cannot loop forever
		for _, parentID := range batchParents(beef, txID, batch) {
			visit(parentID)
		}
		ordered = append(ordered, txID)
	}
	for _, txID := range txIDs {
		visit(txID)
	}
	return ordered
}

// batchParents returns the txids spent by txID that are also subjects of the same
// batch.
func batchParents(beef *transaction.Beef, txID string, batch map[string]struct{}) []string {
	tx := BeefTx(beef, txID)
	if tx == nil {
		return nil
	}
	var parents []string
	for _, in := range tx.Inputs {
		if in.SourceTXID == nil {
			continue
		}
		parentID := in.SourceTXID.String()
		if _, ok := batch[parentID]; ok {
			parents = append(parents, parentID)
		}
	}
	return parents
}

// BeefTx looks a subject transaction up in a beef by its hex txid.
func BeefTx(beef *transaction.Beef, txID string) *transaction.Transaction {
	hash, err := chainhash.NewHashFromHex(txID)
	if err != nil {
		return nil
	}
	return beef.FindTransactionByHash(hash)
}
